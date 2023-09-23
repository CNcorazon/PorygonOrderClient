package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"horizon/Utils"
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
	//priFile, _ := os.Create("ec-pri.pem")
	//pubFile, _ := os.Create("ec-pub.pem")
	//if err := generateKey(priFile, pubFile); err != nil {
	//	log.Println(err)
	//	os.Exit(1)
	//}
	//// 加载私匙公匙
	//if err := loadKey(); err != nil {
	//	log.Println(err)
	//	os.Exit(1)
	//}
}

const (
	HTTPURL = "http://127.0.0.1:8088"
	WSURL   = "ws://127.0.0.1:8088"
	// HTTPURL           = "http://172.18.166.60:8800"
	// WSURL             = "ws://http://172.18.166.60:8800"
	blockTransaction      = "/block/transaction"
	blockAccount          = "/block/account"
	blockUploadProposal   = "/block/uploadproposal"
	blockUploadLeader     = "/block/uploadleader"
	blockGetProposalBlock = "/block/proposalBlock"
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

	var (
		heightOld int
		heightNew int
	)
	for {
		heightNew = request.HeightRequest(HTTPURL, heightNum)
		if heightNew == heightOld && heightNew != 0 {
			//log.Println(heightNew, heightOld)
			continue
		}
		log.Printf("申请加入系统")
		// 加入随机性
		RandomSleep(1000)
		// 发起请求获取当前可以加入的shard
		_, flag1, flag2 := request.ShardRequest(HTTPURL, shardNum)
		if !flag2 {
			log.Printf("服务器尚未开启")
			time.Sleep(1 * time.Second)
			continue
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
		// conn为空表示共识委员会已满
		if conn == nil {
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("登记完成")
		var (
			metaMessage  model.MessageMetaData
			idList       []string
			id           string
			proBlockList []structure.ProposalBlock
		)
		for {
			log.Printf("%v在等待消息,已收到%v个proposalblock", id, len(proBlockList))
			err := conn.ReadJSON(&metaMessage)
			if err != nil {
				fmt.Println(err)
				break
			}
			//log.Printf("%v接收到了消息，类型为%v", id, metaMessage.MessageType)

			if metaMessage.MessageType == 0 {
				var idMsg model.MessageReady
				err := json.Unmarshal(metaMessage.Message, &idMsg)
				if err != nil {
					log.Printf("err")
					break
				}
				id = idMsg.PersonalID
				idList = idMsg.IdList
				shard := uint(0)
				log.Println("---------------开始共识---------------")
				log.Println("---------------开始下载已见证过的交易列表，生成proposal---------------")
				//（2000个交易1个batch，使用聚合签名（40000/2000 * SHARDNUM =200个）+公钥索引（每个分片节点数量*shardnum））
				// pub, _ := ioutil.ReadFile("ec-pub.pem")
				// 共识委员会下载交易，生成proposal
				proposal := request.RequestTransaction(shard, HTTPURL, blockTransaction) // 764bytes * 19
				accList := request.RequestAccount(shard, HTTPURL, blockAccount)          // 56300bytes/100accounts

				proposalBlock := currencyControl(id, idList, proposal, accList)

				request.MultiCastProposal(HTTPURL, blockUploadProposal, proposalBlock)
			}
			if metaMessage.MessageType == 11 {
				var proBlock structure.ProposalBlock
				log.Println(len(metaMessage.Message))
				err := json.Unmarshal(metaMessage.Message, &proBlock)
				if err != nil {
					log.Printf("err")
					break
				}
				proBlockList = append(proBlockList, proBlock)
			}
			if len(proBlockList) == len(idList) {
				heightOld = request.HeightRequest(HTTPURL, heightNum)
				LeaderProblock := Utils.FindLeader(proBlockList)
				res := request.UploadLeaderProblock(HTTPURL, blockUploadLeader, LeaderProblock)
				log.Println(res)
				break
			}
		}
		err := conn.Close()
		if err != nil {
			continue
		}
		//time.Sleep(3 * time.Second)
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
	if block.Bytes != nil && len(block.Bytes) > 0 {
		// 切片不为空且长度大于零
		PriKey, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return err
		}
	} else {
		// 切片为空或长度为零
		// 执行相应的错误处理逻辑
		fmt.Println("block.Bytes is nil or empty")
		return nil
	}

	// 解码公匙
	block, _ = pem.Decode(pub)
	// 反序列化公匙
	var i interface{}
	if block.Bytes != nil && len(block.Bytes) > 0 {
		i, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return err
		}
	} else {
		// 切片为空或长度为零
		// 执行相应的错误处理逻辑
		fmt.Println("block.Bytes is nil or empty")
		return nil
	}
	// PubKey = (*ecdsa.PublicKey)(i)
	var ok bool
	PubKey, ok = i.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("the public conversion error")
	}
	return nil
}

func RandomSleep(n int) {
	rand.Seed(time.Now().UnixNano())
	Random1 := rand.Intn(n)
	time.Sleep(time.Duration(Random1) * time.Millisecond)
}

func currencyControl(id string, idList []string, proposals []structure.Proposal, accList model.BlockAccountResponse) structure.ProposalBlock {
	Random := rand.Intn(100000)
	lockedAccounts := getLockedAccountUnion()
	fmt.Println("涉及到的所有账户：")
	proposalBlock := structure.ProposalBlock{
		Id:            id,
		IdList:        idList,
		Height:        int(proposals[0].Height),
		LockedAccount: lockedAccounts,
		Vrf:           Random,
		Root:          accList.GSRoot,
		ProposalList:  proposals,
	}
	return proposalBlock
}

func getLockedAccountUnion() []int {
	//// 使用 map 来确保结果中的 Id 是唯一的
	uniqueIds0 := make(map[int]struct{})
	uniqueIds1 := make(map[int]struct{})
	uniqueIds2 := make(map[int]struct{})

	req := model.GetProposalRequest{
		Height:   request.HeightRequest(HTTPURL, heightNum),
		Identity: "order",
	}
	res := request.GetProposalBlock(HTTPURL, blockGetProposalBlock, req)
	log.Println("proposalblock个数：", len(res.ProposalBlocks))
	proposals1 := make([]structure.Proposal, 0)
	proposals2 := make([]structure.Proposal, 0)
	proposals3 := make([]structure.Proposal, 0)

	if len(res.ProposalBlocks) == 3 {
		proposals1 = res.ProposalBlocks[2].Body.TransactionLists
		proposals2 = res.ProposalBlocks[1].Body.TransactionLists
		proposals3 = res.ProposalBlocks[0].Body.TransactionLists
	} else if len(res.ProposalBlocks) == 2 {
		proposals1 = res.ProposalBlocks[1].Body.TransactionLists
		proposals2 = res.ProposalBlocks[0].Body.TransactionLists
	} else if len(res.ProposalBlocks) == 1 {
		proposals1 = res.ProposalBlocks[0].Body.TransactionLists
	}

	// 前第一个区块中涉及的所有账户From和To都是禁止访问状态
	// 前第二、三个区块中superTransaction涉及的To账户是禁止访问状态
	for _, proposal := range proposals1 {
		accountId := getRelatedAccount(proposal)
		for _, InterAccountId := range accountId[0] {
			uniqueIds0[InterAccountId] = struct{}{}
		}
		for _, CrossAccountId := range accountId[1] {
			uniqueIds1[CrossAccountId] = struct{}{}
		}
		for _, SuperAccountId := range accountId[2] {
			uniqueIds2[SuperAccountId] = struct{}{}
		}
	}
	for _, proposal := range proposals2 {
		accountId := getRelatedAccount(proposal)
		for _, SuperAccountId := range accountId[2] {
			uniqueIds2[SuperAccountId] = struct{}{}
		}
	}
	for _, proposal := range proposals3 {
		accountId := getRelatedAccount(proposal)
		for _, SuperAccountId := range accountId[2] {
			uniqueIds2[SuperAccountId] = struct{}{}
		}
	}
	// 将唯一的 Id 放入结果切片中
	var accountIds []int
	for id := range uniqueIds0 {
		accountIds = append(accountIds, id)
	}
	for id := range uniqueIds1 {
		accountIds = append(accountIds, id)
	}
	for id := range uniqueIds2 {
		accountIds = append(accountIds, id)
	}
	return accountIds
}

func getRelatedAccount(proposal structure.Proposal) [][]int {
	var accountIds [][]int
	for len(accountIds) <= 3 {
		accountIds = append(accountIds, make([]int, 0))
	}

	for _, ib := range proposal.InternalBatch {
		accountIds[0] = append(accountIds[0], ib.RelatedAccount...)
	}

	for _, cb := range proposal.CrossBatch {
		accountIds[1] = append(accountIds[1], cb.RelatedAccount...)
	}

	for _, cb := range proposal.SuperBatch {
		accountIds[2] = append(accountIds[2], cb.RelatedAccount...)
	}

	return accountIds
}
