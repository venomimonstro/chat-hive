package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type result struct {
	Duration time.Duration
	Status   int
	Err      error
}

func main() {
	mode := flag.String("mode", "health", "health|messages|websocket")
	base := flag.String("base", env("CHAT_LOAD_BASE_URL", "http://localhost:8080"), "API base URL")
	requests := flag.Int("requests", 200, "total HTTP requests")
	concurrency := flag.Int("concurrency", 20, "concurrent workers/connections")
	flag.Parse()

	access := strings.TrimSpace(os.Getenv("CHAT_LOAD_ACCESS_TOKEN"))
	chatID := strings.TrimSpace(os.Getenv("CHAT_LOAD_CHAT_ID"))

	switch *mode {
	case "health":
		runHTTP(*base+"/health/live", http.MethodGet, "", "", *requests, *concurrency)
	case "messages":
		if access == "" || chatID == "" { fatal("CHAT_LOAD_ACCESS_TOKEN and CHAT_LOAD_CHAT_ID are required") }
		runMessageBurst(*base, access, chatID, *requests, *concurrency)
	case "websocket":
		if access == "" { fatal("CHAT_LOAD_ACCESS_TOKEN is required") }
		runWebSockets(*base, access, *concurrency)
	default:
		fatal("unknown mode")
	}
}

func runHTTP(target, method, token, body string, total, concurrency int) {
	jobs := make(chan struct{})
	results := make(chan result, total)
	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	started := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				begin := time.Now()
				req, err := http.NewRequest(method, target, strings.NewReader(body))
				if err == nil && token != "" { req.Header.Set("Authorization", "Bearer "+token) }
				if err == nil && body != "" { req.Header.Set("Content-Type", "application/json") }
				if err != nil { results <- result{Duration:time.Since(begin),Err:err}; continue }
				res, err := client.Do(req)
				status := 0
				if res != nil { status=res.StatusCode; _,_=io.Copy(io.Discard,res.Body); _=res.Body.Close() }
				results <- result{Duration:time.Since(begin),Status:status,Err:err}
			}
		}()
	}
	go func(){ for i:=0;i<total;i++{jobs<-struct{}{}};close(jobs);wg.Wait();close(results) }()
	printResults(results,total,time.Since(started))
}

func runMessageBurst(base, token, chatID string, total, concurrency int) {
	jobs := make(chan int)
	results := make(chan result,total)
	client:=&http.Client{Timeout:10*time.Second}
	var wg sync.WaitGroup
	started:=time.Now()
	for i:=0;i<concurrency;i++{
		wg.Add(1)
		go func(){
			defer wg.Done()
			for index:=range jobs{
				payload,_:=json.Marshal(map[string]string{"client_message_id":newUUID(),"body":fmt.Sprintf("load-test-%d",index)})
				begin:=time.Now()
				req,err:=http.NewRequest(http.MethodPost,strings.TrimRight(base,"/")+"/api/v1/chats/"+url.PathEscape(chatID)+"/messages",bytes.NewReader(payload))
				if err==nil{req.Header.Set("Authorization","Bearer "+token);req.Header.Set("Content-Type","application/json")}
				if err!=nil{results<-result{Duration:time.Since(begin),Err:err};continue}
				res,err:=client.Do(req);status:=0
				if res!=nil{status=res.StatusCode;_,_=io.Copy(io.Discard,res.Body);_=res.Body.Close()}
				results<-result{Duration:time.Since(begin),Status:status,Err:err}
			}
		}()
	}
	go func(){for i:=0;i<total;i++{jobs<-i};close(jobs);wg.Wait();close(results)}()
	printResults(results,total,time.Since(started))
}

func runWebSockets(base, token string, count int) {
	type wsResult struct{ connect time.Duration; err error }
	results:=make(chan wsResult,count)
	started:=time.Now()
	var wg sync.WaitGroup
	for i:=0;i<count;i++{
		wg.Add(1)
		go func(){
			defer wg.Done()
			begin:=time.Now()
			ticket,err:=issueTicket(base,token)
			if err!=nil{results<-wsResult{err:err};return}
			wsURL,err:=toWS(base,ticket)
			if err!=nil{results<-wsResult{err:err};return}
			ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
			conn,_,err:=websocket.Dial(ctx,wsURL,&websocket.DialOptions{HTTPHeader:http.Header{"Origin":[]string{browserOrigin(base)}}})
			if err!=nil{results<-wsResult{err:err};return}
			defer conn.Close(websocket.StatusNormalClosure,"load test done")
			var ready map[string]any
			if err:=wsjson.Read(ctx,conn,&ready);err!=nil{results<-wsResult{err:err};return}
			results<-wsResult{connect:time.Since(begin)}
			<-ctx.Done()
		}()
	}
	go func(){wg.Wait();close(results)}()
	latencies:=make([]time.Duration,0,count);failures:=0
	for item:=range results{if item.err!=nil{failures++;continue};latencies=append(latencies,item.connect)}
	sort.Slice(latencies,func(i,j int)bool{return latencies[i]<latencies[j]})
	fmt.Printf("websocket connections=%d successful=%d failed=%d wall=%s",count,len(latencies),failures,time.Since(started))
	if len(latencies)>0{fmt.Printf(" p50=%s p95=%s p99=%s",percentile(latencies,.50),percentile(latencies,.95),percentile(latencies,.99))}
	fmt.Println()
}

func issueTicket(base,token string)(string,error){
	req,err:=http.NewRequest(http.MethodPost,strings.TrimRight(base,"/")+"/api/v1/realtime/ticket",nil);if err!=nil{return "",err}
	req.Header.Set("Authorization","Bearer "+token)
	res,err:=(&http.Client{Timeout:5*time.Second}).Do(req);if err!=nil{return "",err};defer res.Body.Close()
	if res.StatusCode!=http.StatusCreated{return "",fmt.Errorf("ticket status %d",res.StatusCode)}
	var payload struct{Ticket string `json:"ticket"`};if err:=json.NewDecoder(res.Body).Decode(&payload);err!=nil{return "",err};if payload.Ticket==""{return "",fmt.Errorf("empty ticket")};return payload.Ticket,nil
}

func toWS(base,ticket string)(string,error){u,err:=url.Parse(base);if err!=nil{return "",err};if u.Scheme=="https"{u.Scheme="wss"}else{u.Scheme="ws"};u.Path="/api/v1/realtime";u.RawQuery=url.Values{"ticket":[]string{ticket}}.Encode();return u.String(),nil}
func browserOrigin(base string)string{u,err:=url.Parse(base);if err!=nil{return base};u.Path="";u.RawQuery="";u.Fragment="";return strings.TrimRight(u.String(),"/")}

func printResults(results <-chan result,total int,wall time.Duration){latencies:=make([]time.Duration,0,total);statuses:=map[int]int{};var failures atomic.Int64;for item:=range results{if item.Err!=nil{failures.Add(1);continue};statuses[item.Status]++;latencies=append(latencies,item.Duration)};sort.Slice(latencies,func(i,j int)bool{return latencies[i]<latencies[j]});fmt.Printf("requests=%d completed=%d failed=%d wall=%s rps=%.1f statuses=%v",total,len(latencies),failures.Load(),wall,float64(len(latencies))/wall.Seconds(),statuses);if len(latencies)>0{fmt.Printf(" p50=%s p95=%s p99=%s",percentile(latencies,.50),percentile(latencies,.95),percentile(latencies,.99))};fmt.Println()}
func percentile(values []time.Duration,p float64)time.Duration{if len(values)==0{return 0};index:=int(float64(len(values)-1)*p);if index<0{index=0};if index>=len(values){index=len(values)-1};return values[index]}
func newUUID()string{raw:=make([]byte,16);if _,err:=rand.Read(raw);err!=nil{panic(err)};raw[6]=(raw[6]&0x0f)|0x40;raw[8]=(raw[8]&0x3f)|0x80;value:=hex.EncodeToString(raw);return value[0:8]+"-"+value[8:12]+"-"+value[12:16]+"-"+value[16:20]+"-"+value[20:32]}
func env(key,fallback string)string{if value:=strings.TrimSpace(os.Getenv(key));value!=""{return value};return fallback}
func fatal(message string){fmt.Fprintln(os.Stderr,message);os.Exit(2)}
