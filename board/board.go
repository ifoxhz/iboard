
package main
import (
    "log"
    "net/http"
    "fmt"
    "strconv"
    "gopkg.in/redis.v3"
    //"github.com/PuerkitoBio/goquery"
    "os"
    "time"
    "runtime"
    "encoding/json"
  //  "crypto/md5"
    "github.com/chanxuehong/wechat.v2/mp/core"
    "github.com/chanxuehong/wechat.v2/mp/menu"
    "github.com/chanxuehong/wechat.v2/mp/message/callback/request"
    "github.com/chanxuehong/wechat.v2/mp/message/callback/response"
    "github.com/chanxuehong/wechat.v2/mp/media"
)
type Article struct {
    Id  string
    Title string
   Author string
    Url string
    Location string
    Time time.Time
}

const (
    wxAppId     = "wxeff792e850f8af34"
    wxAppSecret = "08cf2d7596710cf7edc368e6e36f0608"

    wxOriId         = "gh_9ad0163bc42f"
    wxToken         = "TIUXIUHONGWEIXIN"
    wxEncodedAESKey = "xuddmSvZQQ5uASryNHIqilbVfXzL2vVQWUaYBvxm5Lu"
    TIUXIUHONGWEIXINWEBSITE = "http://iothill.com/board/wxtuiboard/?id="
    BOARD_CUR_ARC_LIST = "IBOARD:CURARTICLE"
    BOARD_LEGACY_Week_ARC_HASH = "IBORAD:WEEKARTICLE:"
)


var (
    // 下面两个变量不一定非要作为全局变量, 根据自己的场景来选择.
    msgHandler core.Handler
    msgServer  *core.Server
    wxClient *core.Client
     GRdb  * redis.Client = nil
     RedisOption =  redis.Options {
        Addr: "127.0.0.1:6379",
        Password:"",
        DB: 0,
    }
    Logger *log.Logger = nil
)

func init() {

  Logfile ,_ := os.OpenFile("/var/iboard/log/iboard.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE,660)
  Logger = log.New(Logfile, "[board]", log.LstdFlags) //os.Stdout
    mux := core.NewServeMux()
    mux.DefaultMsgHandleFunc(defaultMsgHandler)
    mux.DefaultEventHandleFunc(defaultEventHandler)
    mux.MsgHandleFunc(request.MsgTypeText, textMsgHandler)
    mux.EventHandleFunc(menu.EventTypeClick, menuClickEventHandler)
    mux.EventHandleFunc(request.EventTypeSubscribe, SubScribeEventHandler)

    msgHandler = mux
    msgServer = core.NewServer(wxOriId, wxAppId, wxToken, wxEncodedAESKey, msgHandler, nil)

    AccessToken := core.NewDefaultAccessTokenServer(wxAppId, wxAppSecret ,nil)
    wxClient = core.NewClient(AccessToken,nil)

    if (GRdb == nil){
        GRdb =  redis.NewClient(&RedisOption)
    }

    Logger.Printf("board start\n")

}

func textMsgHandler(ctx *core.Context) {
  msg := request.GetText(ctx.MixedMsg)
  var id =  0
  if i, err := strconv.Atoi(msg.Content); err != nil {
       id = -1
  }else {
    id = i
  }
  //id is -1, can scan match  key-words  contenet
  if  0 <= id && id <= 7 {
     FeedBackDefault(id, ctx)
  }else {
    FeedBackKeyWorkd(string(id), ctx)
  }
}

func defaultMsgHandler(ctx *core.Context) {
    Logger.Printf("收到消息:\n%s\n", ctx.MsgPlaintext)
    msg := request.GetText(ctx.MixedMsg)
    ctx.RawResponse(msg)
}

func menuClickEventHandler(ctx *core.Context) {
    Logger.Printf("收到菜单 click 事件:\n%s\n", ctx.MsgPlaintext)
    restxt := "%s, 您好！请发送0 获取当天推荐，1，2, 3, 4 等数字获取过去一周的推荐"
    Logger.Printf("subcribe 事件:\n%s\n", ctx.MsgPlaintext)
    event := request.GetSubscribeEvent(ctx.MixedMsg)
    txt := fmt.Sprintf(restxt,event.FromUserName)
    resp := response.NewText(event.FromUserName, event.ToUserName, event.CreateTime, txt)
    ctx.RawResponse(resp) // 明文回复
    //ctx.AESResponse(resp, 0, "", nil) // aes密文回复
}

func SubScribeEventHandler(ctx *core.Context) {
    restxt := "%s, 您好！请发送0 获取当天推荐，1，2, 3, 4 等数字获取过去一周的推荐"
    Logger.Printf("subcribe 事件:\n%s\n", ctx.MsgPlaintext)
    event := request.GetSubscribeEvent(ctx.MixedMsg)
    resp := response.NewText(event.FromUserName, event.ToUserName, event.CreateTime, restxt)
    ctx.RawResponse(resp) // 明文回复
    //ctx.AESResponse(resp, 0, "", nil) // aes密文回复
}

func defaultEventHandler(ctx *core.Context) {
    Logger.Printf("收到事件:\n%s\n", ctx.MsgPlaintext)
    ctx.NoneResponse()
}

func wxCallbackHandler(w http.ResponseWriter, r *http.Request) {
    //Logger.Println("wx", r)
    msgServer.ServeHTTP(w, r, nil)
}
func CreateMenu(){
  AccessToken := core.NewDefaultAccessTokenServer(wxAppId, wxAppSecret ,nil)
  wxclient := core.NewClient(AccessToken,nil)
  //fmt.Println(wxclient)
  Bu1 := menu.Button{Name:"推荐"}
  var buitem =make([]menu.Button,1)
  buitem[0] = Bu1
  mitem := menu.Menu{buitem,nil, 16}
  err := menu.Create(wxclient,&mitem)
  if err != nil{
     fmt.Println(err)
   }
   fmt.Printf("Succesed to ctreate menu")
}

func PostTempMedia() {
    art := make([]media.Article,2)
    art[0] = media.Article {Title:"kantu", Digest:"I am old", Content:`<a href="www.iothill.com">link</a>`}
    art[1] = media.Article {Title:"交警", Digest:"违法", Content:"www.oschina.com"}
    new  := media.News {art}

    mdi, err := media.UploadNews(wxClient,&new)
    if  err != nil {
      fmt.Println(err)
    }else{
      fmt.Println(mdi)
    }
}

func  FeedBackDefault( id int, ctx *core.Context ) {
  msg := request.GetText(ctx.MixedMsg)
  news := make([]response.Article,0)
  key := ""
  if id == 0{
    key = BOARD_CUR_ARC_LIST
  }else {
    weekday := []string {
    "Monday",
    "Tuesday",
    "Wednesday",
    "Thursday",
    "Friday",
    "Saturday",
    "Sunday",
  }
    key = BOARD_LEGACY_Week_ARC_HASH+weekday[id-1]
  }

  artlist, err := GRdb.LRange(key,0,-1).Result()
  if  err != nil {
        Logger.Printf("Failed to Lrange db  : %s", err)
        news[0] = response.Article {Title:"刚才有点忙呢，请在试一试！"}
   }else{
      for _, v := range artlist{
        var art Article
        err := json.Unmarshal([]byte(v), &art)
        if err != nil{
          Logger.Printf("Failed to unMarshal : %s %s", v, err)
          continue
        }
       var  tmpart response.Article
        tmpart.Title = art.Title
        tmpart.URL =  TIUXIUHONGWEIXINWEBSITE+art.Id
        news = append(news, tmpart)
      }
   }

    if len(news) > 5 {
        news = news[0:4]
    }

  resp := response.NewNews(msg.FromUserName, msg.ToUserName, msg.CreateTime, news)
  ctx.RawResponse(resp) // 明文回复
    //ctx.Response(resp, 0, "", nil) // aes密文回复
}
func FeedBackKeyWorkd( key string,ctx *core.Context){
  msg := request.GetText(ctx.MixedMsg)
  art := make([]response.Article,1)
  art[0] = response.Article {Title:"退休红", Description:"请发送0 获取当天推荐, 1,2, 3, 4 等数字获取过去一周的推荐"}
  //art[1] = response.Article {Title:"2234", Description:"新故事", URL:"www.pm25.com"}
  resp := response.NewNews(msg.FromUserName, msg.ToUserName, msg.CreateTime, art)
  ctx.RawResponse(resp)
}

func main() {
  var MULTICORE int = runtime.NumCPU() //number of core
  runtime.GOMAXPROCS(MULTICORE) //running in multicore

  http.HandleFunc("/", wxCallbackHandler)
  http.ListenAndServe(":7000", nil)
  Logger.Printf("board exit")
}
