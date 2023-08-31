package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"horizon/logger"
	"horizon/model"
	"horizon/request"
	"horizon/structure"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/pochard/commons/randstr"
)

//随机熵，用于加密安全
// var randSign = randstr.RandomAlphanumeric(43)

//随机key，用于创建公钥和私钥
var randKey = randstr.RandomAlphanumeric(40)

var PriKey *ecdsa.PrivateKey
var PubKey *ecdsa.PublicKey

func init() {

	// 初始化生成私匙公匙
	priFile, _ := os.Create("ec-pri.pem")
	pubFile, _ := os.Create("ec-pub.pem")
	if err := generateKey(priFile, pubFile); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	// 加载私匙公匙
	if err := loadKey(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

const (
	HTTPURL = "http://127.0.0.1:8088"
	WSURL   = "ws://127.0.0.1:8088"
	// HTTPURL           = "http://172.18.166.60:8800"
	// WSURL             = "ws://http://172.18.166.60:8800"
	blockTransaction    = "/block/transaction"
	blockAccount        = "/block/account"
	blockUpload         = "/block/upload"
	blockUploadProposal = "/block/uploadproposal"
	// blockWitness      = "/block/witness"
	// blockWitness_2    = "/block/witness_2"
	// blockTxValidation = "/block/validate"
	// blockUploadRoot   = "/block/uploadroot"

	shardNum       = "/shard/shardNum"
	consensusflag  = "/shard/flag"
	register       = "/shard/register"
	muliticastconn = "/shard/multicast"
	multicastblock = "/shard/block"
	sendtvote      = "/shard/vote"
	heightNum      = "/shard/height"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	for {
		log.Printf("申请加入系统")
		start := time.Now()
		// 加入随机性
		RandomSleep(1000)
		// 发起请求获取当前可以加入的shard
		_, flag1, flag2 := request.ShardRequest(HTTPURL, shardNum)
		if !flag2 {
			log.Printf("服务器尚未开启")
			time.Sleep(1 * time.Second)
		}

		if !flag1 {
			log.Printf("当前没有分片需要节点")
			time.Sleep(1 * time.Second)
			continue
		}
		log.Printf("成功进入系统")

		// 加入随机性
		RandomSleep(1500)

		log.Println("---------------开始登记节点信息---------------")
		conn := request.RegisterWSRequest(WSURL, register)
		ReshardTime := time.Since(start)
		// conn为空表示共识委员会已满
		if conn == nil {
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("登记完成")
		var id, winid string
		var shard uint
		var winflag bool
		var consensus_flag bool
		var metamessage model.MessageMetaData
		// 根据传来的消息获取本客户端的Id和本轮的胜者id
		conn.ReadJSON(&metamessage)
		if metamessage.MessageType == 1 {
			logger.BandWidthLogger.Println(len(metamessage.Message))
			var iswin model.MessageIsWin
			err := json.Unmarshal(metamessage.Message, &iswin)
			if err != nil {
				log.Printf("err")
				return
			}
			consensus_flag = iswin.IsConsensus
			winflag = iswin.IsWin
			id = iswin.PersonalID
			winid = iswin.WinID
			shard = uint(iswin.Shardnum)
		}

		// 进行区块共识
		if consensus_flag && winflag {
			log.Println("---------------开始共识---------------")
			log.Println("---------------开始下载已见证过的交易列表（2000个交易1个batch，使用聚合签名（40000/2000 * SHARDNUM =200个）+公钥索引（每个分片节点数量*shardnum）），生成proposal---------------")
			consensusstart := time.Now()
			// pub, _ := ioutil.ReadFile("ec-pub.pem")
			// 共识委员会下载交易，生成proposal
			Proposal := request.RequestTransaction(shard, HTTPURL, blockTransaction, id) // 764bytes * 19
			time.Sleep(1 * time.Second)
			/* proposal签名 */
			// list2marshal := structure.TransactionBlock{
			// 	// Id:             id,
			// 	Height:         txlist.Height,
			// 	InternalList:   txlist.InternalList,
			// 	CrossShardList: txlist.CrossShardList,
			// 	SuperList:      txlist.RelayList,
			// }
			// jsonString, _ := json.Marshal(list2marshal)
			// hash := sha256.Sum256(jsonString)
			// sign := witness(hash[:], pub)
			// txlist.Sign = sign
			/* 结束 */

			// 广播proposal
			request.MultiCastProposal(shard, HTTPURL, blockUploadProposal, Proposal[0], id)
			time.Sleep(1 * time.Second)
			/*
				应该用websocket接受广播的信息，这边直接下载代替
			*/
			Proposals := request.RequestTransaction(0, HTTPURL, blockTransaction, id) // 764bytes * 19
			time.Sleep(2 * time.Second)

			log.Println("---------------获胜者下载其他相关信息，生成区块以进行BBA共识---------------")
			// 请求账户的状态
			// 和各分片树根签名 accList.GSRoot
			accList := request.RequestAccount(0, HTTPURL, blockAccount) // 56300bytes/100accounts
			time.Sleep(536 * time.Millisecond)
			state := structure.MakeStateWithAccount(0, accList.AccountList, accList.GSRoot)
			// 验证树根签名
			time.Sleep(time.Duration(structure.NodeNum*structure.SIGN_VERIFY_TIME*structure.ShardNum) * time.Microsecond / structure.CORE)
			newBlock := structure.MakeBlock(Proposals, state, Proposals[0].Height, accList.GSRoot)

			log.Println("---------------获胜者BBA---------------")
			/* 本应该是所有委员会成员上传相同的block，即request.SendVote,由服务器收集到足够多的投票数即可完成共识。（待实现）*/
			// 这边改成由leader收集投票，上传最终的block，完成共识。
			blockPointer := &newBlock
			// 用这个模拟收集投票的过程
			RandomSleep(3000)
			blockPointer.Header.Vote = blockPointer.Header.Vote + uint(structure.ProposerNum) - 1
			finalBlock := *blockPointer
			res1 := request.UploadBlock(0, finalBlock, winid, HTTPURL, blockUpload) //1136bytes
			time.Sleep(1 * time.Millisecond)
			consensustime := time.Since(consensusstart)
			log.Println("---------------共识结束--------------")
			log.Printf("分片%v%v,当前链的高度为%v", res1.Shard, res1.Message, res1.Height)
			str := fmt.Sprintf("重分片时间:%v,consensus:%v", ReshardTime, consensustime)
			//写入文件
			dstFile, err := os.OpenFile("/Users/xiading/Library/Mobile Documents/com~apple~CloudDocs/学习/中山大学/论文代码/go-project/WinnerConsensus.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			defer dstFile.Close()
			dstFile.WriteString(str + "\n")
			time.Sleep(2 * time.Second)
		} else if consensus_flag && !winflag {
			log.Println("---------------开始共识---------------")
			log.Println("---------------开始下载已见证过的交易列表（2000个交易1个batch，使用聚合签名（90000/2000=45个）+公钥索引（45*每个分片节点数量）），生成proposal---------------")
			consensusstart := time.Now()
			// pub, _ := ioutil.ReadFile("ec-pub.pem")
			// 共识委员会下载交易，生成proposal
			Proposal := request.RequestTransaction(shard, HTTPURL, blockTransaction, id) // 764bytes * 19
			time.Sleep(1 * time.Second)
			/* proposal签名 */
			// list2marshal := structure.TransactionBlock{
			// 	// Id:             id,
			// 	Height:         txlist.Height,
			// 	InternalList:   txlist.InternalList,
			// 	CrossShardList: txlist.CrossShardList,
			// 	SuperList:      txlist.RelayList,
			// }
			// jsonString, _ := json.Marshal(list2marshal)
			// hash := sha256.Sum256(jsonString)
			// sign := witness(hash[:], pub)
			// txlist.Sign = sign
			/* 结束 */

			// 广播proposal
			request.MultiCastProposal(shard, HTTPURL, blockUploadProposal, Proposal[0], id)
			time.Sleep(1 * time.Second)
			/*
				应该用websocket接受广播的信息，这边直接下载代替
			*/
			Proposals := request.RequestTransaction(0, HTTPURL, blockTransaction, id) // 764bytes * 19
			time.Sleep(2 * time.Second)

			log.Println("---------------获胜者下载其他相关信息，生成区块以进行BBA共识---------------")
			// 请求账户的状态
			// 和各分片树根签名 accList.GSRoot
			accList := request.RequestAccount(0, HTTPURL, blockAccount) // 56300bytes/100accounts
			time.Sleep(536 * time.Millisecond)
			state := structure.MakeStateWithAccount(0, accList.AccountList, accList.GSRoot)
			// 验证树根签名
			time.Sleep(time.Duration(structure.NodeNum*structure.SIGN_VERIFY_TIME*structure.ShardNum) * time.Microsecond / structure.CORE)
			newBlock := structure.MakeBlock(Proposals, state, Proposals[0].Height, accList.GSRoot)

			log.Println("---------------非获胜者BBA---------------")
			/* 本应该是所有委员会成员上传相同的block，即request.SendVote,由服务器收集到足够多的投票数即可完成共识。（待实现）*/
			// 这边改成向leader投票，完成共识。
			RandomSleep(3000)
			resp := request.SendVote(shard, int(newBlock.Header.Height), winid, id, true, HTTPURL, sendtvote)
			log.Println(resp.Message)
			consensustime := time.Since(consensusstart)
			log.Println("---------------共识结束--------------")
			str := fmt.Sprintf("重分片时间:%v, consensus:%v", ReshardTime, consensustime)
			//写入文件
			dstFile, err := os.OpenFile("/Users/xiading/Library/Mobile Documents/com~apple~CloudDocs/学习/中山大学/论文代码/go-project/ProposerConsensus.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			defer dstFile.Close()
			dstFile.WriteString(str + "\n")
			time.Sleep(2 * time.Second)
		}
		conn.Close()
		// rand.Seed(time.Now().UnixNano())
		// Random1 := rand.Intn(5000)
		// time.Sleep(time.Duration(Random1) * time.Millisecond)
	}
}

// 生成密匙对
func generateKey(priFile, pubFile *os.File) error {
	lenth := len(randKey)
	if lenth < 224/8 {
		return errors.New("私钥长度太短，至少为36位！")
	}
	// 根据随机密匙的长度创建私匙
	var curve elliptic.Curve
	if lenth > 521/8+8 {
		curve = elliptic.P521()
	} else if lenth > 384/8+8 {
		curve = elliptic.P384()
	} else if lenth > 256/8+8 {
		curve = elliptic.P256()
	} else if lenth > 224/8+8 {
		curve = elliptic.P224()
	}
	// 生成私匙
	priKey, err := ecdsa.GenerateKey(curve, strings.NewReader(randKey))
	if err != nil {
		return err
	}
	// *****************保存私匙*******************
	// 序列化私匙
	priBytes, err := x509.MarshalECPrivateKey(priKey)
	if err != nil {
		return err
	}
	priBlock := pem.Block{
		Type:  "ECD PRIVATE KEY",
		Bytes: priBytes,
	}
	// 编码私匙,写入文件
	if err := pem.Encode(priFile, &priBlock); err != nil {
		return err
	}
	// *****************保存公匙*******************
	// 序列化公匙
	pubBytes, err := x509.MarshalPKIXPublicKey(&priKey.PublicKey)
	if err != nil {
		return err
	}
	pubBlock := pem.Block{
		Type:  "ECD PUBLIC KEY",
		Bytes: pubBytes,
	}
	// 编码公匙,写入文件
	if err := pem.Encode(pubFile, &pubBlock); err != nil {
		return err
	}
	return nil
}

// 加载私匙公匙
func loadKey() error {
	// 读取密匙
	pri, _ := ioutil.ReadFile("ec-pri.pem")
	pub, _ := ioutil.ReadFile("ec-pub.pem")
	// 解码私匙
	block, _ := pem.Decode(pri)
	var err error
	// 反序列化私匙
	// PriKey.X.MarshalJSON()
	PriKey, err = x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	// 解码公匙
	block, _ = pem.Decode(pub)
	// 反序列化公匙
	var i interface{}
	i, err = x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	// PubKey = (*ecdsa.PublicKey)(i)
	var ok bool
	PubKey, ok = i.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("the public conversion error")
	}
	return nil
}

// func witness(context []byte, pub []byte) structure.PubKeySign {
// 	r, s, err := ecdsa.Sign(strings.NewReader(randSign), PriKey, context)
// 	if err != nil {
// 		log.Println(err)
// 		os.Exit(1)
// 	}

// 	r_byte, _ := r.MarshalJSON()
// 	s_byte, _ := s.MarshalJSON()
// 	sign := structure.PubKeySign{
// 		R: string(r_byte),
// 		S: string(s_byte),
// 	}

// 	return sign
// }

func RandomSleep(n int) {
	rand.Seed(time.Now().UnixNano())
	Random1 := rand.Intn(n)
	time.Sleep(time.Duration(Random1) * time.Millisecond)
}

// //等待一段时间获取投票结果
// voteMap := make(map[string]bool) //记录最终的投票结果
// var wg sync.WaitGroup
// lock := new(sync.Mutex)
// //等待所有的投票结果
// // log.Println(len(conn1))
// metaMessage := make([]model.MessageMetaData, len(conn1))
// //读的时候要读来自除去iswin.shard之外所有分片的数据
// for i := 0; i < len(conn1); i++ {
// 	wg.Add(1)
// 	go func(j int) {
// 		//试试用gofunc()
// 		log.Printf("im here")
// 		conn1[j].ReadJSON(&metaMessage[j])
// 		log.Println("123412123")
// 		// conn.ReadJSON(&metaMessage)
// 		// log.Println(metaMessage.MessageType)
// 		if metaMessage[j].MessageType == 3 {
// 			logger.BandWidthLogger.Println(len(metamessage.Message))
// 			var vote model.SendVoteRequest
// 			err := json.Unmarshal(metaMessage[j].Message, &vote)
// 			if err != nil {
// 				log.Println(err)
// 				return
// 			}
// 			lock.Lock()
// 			log.Println("锁了")
// 			voteMap[vote.PersonalID] = vote.Agree
// 			if vote.Agree {
// 				log.Printf("区块获得了来自%v节点的投票", vote.PersonalID)
// 			}
// 			lock.Unlock()
// 			log.Println("解锁了")
// 		}
// 		defer wg.Done()
// 	}(i)
// }
// wg.Wait()
//最终根据投票结果更新区块接收到的票数目
// for _, value := range voteMap {
// 	if value {
// 		blockPointer.Header.Vote++
// 	}
// }
// MuiltiCastTime := time.Since(time5)
// time6 := time.Now()
//最后提交区块

// //给proposal投票
// var metaMessage model.MessageMetaData
// conn.ReadJSON(&metaMessage)
// if metaMessage.MessageType == 2 {
// fmt.Printf("iam here13.0")
// var blockMessage model.MultiCastBlockRequest
// json.Unmarshal(metaMessage.Message, &blockMessage)
// logger.BandWidthLogger.Println(int(unsafe.Sizeof(metaMessage.MessageType)) + len(metaMessage.Message))
//验证收到的Leader执行生成的区块是否正确
// log.Println(blockMessage.Block.Header.StateRoot.StateRoot)
// log.Println(accList)
// flag := structure.CompareBlocks(blockMessage.Block, newBlock2)
//进行投票

//验证交易列表的签名，即其他移动节点的签名，表示大家都能下载到这些交易
/* 验证签名 */
// TxRoot := make(map[uint]string)
// r := new(big.Int)
// s := new(big.Int)
// for i := 1; i <= structure.ShardNum; i++ {
// 	TxlistCount := make(map[string]int)
// 	for index, PubSign := range TransactionList.Signsmap[uint(i)] {
// 		// 解码公钥
// 		// block := pem.Block{}
// 		// log.Println(PubSign.Pub)
// 		block, _ := pem.Decode(pub)
// 		// 反序列化公钥
// 		var xyz interface{}
// 		var err error
// 		// log.Println(block.Type)
// 		xyz, err = x509.ParsePKIXPublicKey(block.Bytes)
// 	if err != nil {
// 		log.Printf("反序列化公钥失败:%v", err)
// 	}
// 	PubKey, _ = xyz.(*ecdsa.PublicKey)
// 	hash, err := hex.DecodeString(TransactionList.RootString[uint(i)][uint(index)])
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	json.Unmarshal([]byte(PubSign.R), &r)
// 	json.Unmarshal([]byte(PubSign.S), &s)
// 	if ecdsa.Verify(PubKey, hash, r, s) {
// 		TxlistCount[TransactionList.RootString[uint(i)][uint(index)]] += 1
// 		log.Printf("验证成功，交易列表hash为：%v", TxlistCount)
// 		// log.Printf("Root:%v,\npub:%v,\nR:%v,\nS:%v\n", TransactionList.RootString[uint(i)][uint(index)], , PubSign.R, PubSign.S)
// 	} else {
// 		log.Printf("验证失败，相关信息如下")
// 		log.Printf("Root:%v,\nR:%v,\nS:%v\n", TransactionList.RootString[uint(i)][uint(index)], PubSign.R, PubSign.S)
// 	}
// }
// for rootstring, count := range TxlistCount {
// 	if count >= 2*(structure.CLIENT_MAX-structure.ProposerNum)/3 {
// 		TxRoot[uint(i)] = rootstring
// 		break
// 	}
// }
// }
// 下载一个proposal+验证签名的时间,这边直接sleep过去其他proposal的时间
// DownloadTxlistTime := time.Since(consensusstart)
// time.Sleep(DownloadTxlistTime * (structure.ProposerNum/2 - 1))

/* 验证签名 */
// TxRoot := make(map[uint]string)
// r := new(big.Int)
// s := new(big.Int)
// for i := 1; i <= structure.ShardNum; i++ {
// 	TxlistCount := make(map[string]int)
// 	for index, PubSign := range TransactionList.Signsmap[uint(i)] {
// 		// 解码公钥
// 		// block := new(pem.Block)
// 		block, _ := pem.Decode(pub)
// 		// 反序列化公钥
// 		var j interface{}
// 		j, err := x509.ParsePKIXPublicKey(block.Bytes)
// 		if err != nil {
// 			log.Printf("反序列化公钥失败")
// 		}
// 		PubKey1, _ := j.(*ecdsa.PublicKey)
// 		hash, err := hex.DecodeString(TransactionList.RootString[uint(i)][uint(index)])
// 		if err != nil {
// 			fmt.Println(err)
// 			return
// 		}
// 		json.Unmarshal([]byte(PubSign.R), &r)
// 		json.Unmarshal([]byte(PubSign.S), &s)
// 		if ecdsa.Verify(PubKey1, hash, r, s) {
// 			TxlistCount[TransactionList.RootString[uint(i)][uint(index)]] += 1
// 		}
// 	}
// 	for rootstring, count := range TxlistCount {
// 		if count >= 2*(structure.CLIENT_MAX-structure.ProposerNum)/3 {
// 			TxRoot[uint(i)] = rootstring
// 			break
// 		}
// 	}
// }
