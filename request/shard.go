package request

import (
	"bytes"
	"encoding/json"
	"horizon/logger"
	"horizon/model"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

// ShardRequest 返回值分别表示本次被分配到的分片，系统中节点是否已满，HTTP连接是否成功
func ShardRequest(httpUrl string, route string) (uint, bool, bool) {
	URL := httpUrl + route
	request, _ := http.NewRequest("GET", URL, nil)
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Printf("HTTP连接已经断开,等待服务器开启")
		return uint(0), false, false
	}
	body, _ := ioutil.ReadAll(response.Body)
	var res model.ShardNumResponse
	err = json.Unmarshal(body, &res)
	if err != nil {
		return 0, false, false
	}
	shardNum := res.ShardNum

	return shardNum, true, true
}
func HeightRequest(httpUrl string, route string) int {
	// strShard := strconv.Itoa(int(shardnum))
	URL := httpUrl + route
	request, _ := http.NewRequest("GET", URL, nil)
	client := &http.Client{}
	response, _ := client.Do(request)
	body, _ := ioutil.ReadAll(response.Body)

	// 带宽
	logger.UBandWidthLogger.Println(unsafe.Sizeof(URL))
	logger.BandWidthLogger.Println(len(body))

	var res model.HeightResponse
	json.Unmarshal(body, &res)
	return res.Height
}

func RegisterWSRequest(wsurl string, route string) *websocket.Conn {
	rand.Seed(time.Now().UnixNano())
	// str := strconv.Itoa(int(shardnum))
	//Random := rand.Intn(1000)
	//strRand := strconv.Itoa(Random)
	URL := wsurl + route
	var dialer *websocket.Dialer
	conn, _, err := dialer.Dial(URL, nil)

	// 带宽
	logger.UBandWidthLogger.Println(unsafe.Sizeof(URL))
	logger.BandWidthLogger.Println(unsafe.Sizeof(conn))

	if err != nil {
		log.Println(err)
		return nil
	}
	return conn
}

func SendVote(shard uint, height int, winid string, id string, flag bool, httpurl string, route string) model.SendVoteResponse {
	URL := httpurl + route
	data := model.SendVoteRequest{
		Shard:       shard,
		BlockHeight: height,
		WinID:       winid,
		PersonalID:  id,
		Agree:       flag,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
	}
	// log.Printf("节点尝试向分片%v的区块%v进行投票", shard, height)
	request, _ := http.NewRequest("POST", URL, bytes.NewBuffer(jsonData))
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	body, _ := ioutil.ReadAll(response.Body)

	// 带宽
	logger.UBandWidthLogger.Println(len(jsonData))
	logger.BandWidthLogger.Println(len(body))

	var res model.SendVoteResponse
	json.Unmarshal(body, &res)
	return res
}

func MultiCastWSRequest(shardnum uint, wsurl string, route string) *websocket.Conn {
	rand.Seed(time.Now().UnixNano())
	str := strconv.Itoa(int(shardnum))
	// Random := rand.Int()
	// strRand := strconv.Itoa(Random)
	URL := wsurl + route + "/" + str
	var dialer *websocket.Dialer
	conn, _, err := dialer.Dial(URL, nil)

	// 带宽
	logger.UBandWidthLogger.Println(unsafe.Sizeof(URL))
	logger.BandWidthLogger.Println(unsafe.Sizeof(conn))

	if err != nil {
		log.Println(err)
		return nil
	}
	return conn
}
