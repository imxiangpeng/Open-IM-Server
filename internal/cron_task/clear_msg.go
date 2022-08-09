package cronTask

import (
	"Open_IM/pkg/common/config"
	"Open_IM/pkg/common/db"
	"Open_IM/pkg/common/log"
	server_api_params "Open_IM/pkg/proto/sdk_ws"
	"Open_IM/pkg/utils"
	"github.com/golang/protobuf/proto"
	"strconv"
	"strings"
)

const oldestList = 0
const newestList = -1

func ResetUserGroupMinSeq(operationID, groupID, userID string) error {
	return nil
}

func DeleteMongoMsgAndResetRedisSeq(operationID, userID string) error {
	// -1 表示从当前最早的一个开始
	var delMsgIDList []string
	minSeq, err := deleteMongoMsg(operationID, userID, oldestList, &delMsgIDList)
	if err != nil {
		return utils.Wrap(err, "")
	}
	log.NewDebug(operationID, utils.GetSelfFuncName(), "delMsgIDList: ", delMsgIDList)
	err = db.DB.SetUserMinSeq(userID, minSeq)
	return err
}

// recursion
func deleteMongoMsg(operationID string, ID string, index int64, IDList *[]string) (uint32, error) {
	// 从最旧的列表开始找
	msgs, err := db.DB.GetUserMsgListByIndex(ID, index)
	if err != nil {
		return 0, utils.Wrap(err, "GetUserMsgListByIndex failed")
	}
	log.NewDebug(operationID, utils.GetSelfFuncName(), "get msgs: ", msgs.UID)
	for i, msg := range msgs.Msg {
		// 找到列表中不需要删除的消息了
		if msg.SendTime+int64(config.Config.Mongo.DBRetainChatRecords) > utils.GetCurrentTimestampByMill() {
			if len(*IDList) > 0 {
				err := db.DB.DelMongoMsgs(*IDList)
				if err != nil {
					return 0, utils.Wrap(err, "DelMongoMsgs failed")
				}
			}
			minSeq := getDelMaxSeqByIDList(*IDList)
			if i > 0 {
				msgPb := &server_api_params.MsgData{}
				err = proto.Unmarshal(msg.Msg, msgPb)
				if err != nil {
					log.NewError(operationID, utils.GetSelfFuncName(), err.Error(), ID, index)
				} else {
					err = db.DB.ReplaceMsgToBlankByIndex(msgs.UID, i-1)
					if err != nil {
						log.NewError(operationID, utils.GetSelfFuncName(), err.Error(), msgs.UID, i)
						return minSeq, nil
					}
					minSeq = msgPb.Seq - 1
				}
			}
			return minSeq, nil
		}
	}
	*IDList = append(*IDList, msgs.UID)
	// 没有找到 代表需要全部删除掉 继续递归查找下一个比较旧的列表
	seq, err := deleteMongoMsg(operationID, utils.GetSelfFuncName(), index-1, IDList)
	if err != nil {
		return 0, utils.Wrap(err, "deleteMongoMsg failed")
	}
	return seq, nil
}

func getDelMaxSeqByIDList(IDList []string) uint32 {
	if len(IDList) == 0 {
		return 0
	}
	l := strings.Split(IDList[len(IDList)-1], ":")
	index, _ := strconv.Atoi(l[len(l)-1])
	if index == 0 {
		// 4999
		return uint32(db.GetSingleGocMsgNum()) - 1
	} // 5000
	return (uint32(db.GetSingleGocMsgNum()) - 1) + uint32(index*db.GetSingleGocMsgNum())
}