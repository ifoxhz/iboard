
package main
import(
  "gopkg.in/redis.v3"
  //"github.com/PuerkitoBio/goquery"
  "github.com/ifoxhz/go-readability"
  "net/http"
  "log"
  "os"
  "fmt"
  "crypto/md5"
  "io/ioutil"
  "regexp"
  "path/filepath"
  "time"
  "encoding/json"
  "errors"
  "runtime"
  "strings"
)

var (
   RedisOption =  redis.Options {
      Addr: "127.0.0.1:6379",
      Password:"",
      DB: 0,
  }
  GRdb  * redis.Client = nil
  WHALE_URL_LIST =        "WHALE:URL"
  WHALE_ARTICLE_LOCATION = "WHALE:ARTICLE-LOCATION"
  BOARD_CUR_ARC_LIST = "IBOARD:CURARTICLE"
  BOARD_LEGACY_ARC_HASH = "IBORAD:LEGACYARTICLE"
  BOARD_LEGACY_Week_ARC_HASH = "IBORAD:WEEKARTICLE:"
  Logfile *os.File = nil
 	Logger * log.Logger = nil
  RegexpUrl  *regexp.Regexp = nil
)
type Article struct {
    Id  string
    Title string
   Author string
    Url string
    Location string
    Time time.Time
}

func main(){

  var MULTICORE int = runtime.NumCPU() //number of core
        runtime.GOMAXPROCS(MULTICORE) //running in multicore

  Logfile ,_ := os.OpenFile("/var/iboard/log/iboard.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE,660)
  Logger = log.New(Logfile, "[whale]", log.LstdFlags) //os.Stdout
  defer Logfile.Close()

  InitDb()
  result := GRdb.Get(WHALE_ARTICLE_LOCATION)
  if err := result.Err(); err != nil {
        Logger.Printf("Failed to read location from db  : %s", err)
        return
   }
   save_location := result.Val()
    if !CheckFileIsExist(save_location) {
      Logger.Printf("%s is an invald location", save_location)
      return
    }

  for ;; {
    url, err := PopUrl()
    if  err != nil {
      if strings.Contains(err.Error(), "timeout"){
        Logger.Printf("The task is null, exist")
        return
      }else{
        Logger.Printf(err.Error())
        continue
      }


   }
   // trim key
   url  = url[1:]
   for _, v  := range url {
        if arc, err := arc90(v, save_location); err == nil{
             SaveArc(*arc)
        }else{
                  Logger.Println(err)
        }
   }//range
 }//forever
}

func  PopUrl() ([]string, error){
  result := GRdb.BLPop(30, WHALE_URL_LIST)
  if err := result.Err(); err != nil {
      if !strings.Contains(err.Error(), "timeout"){
          Logger.Printf("Failed to pop url from db  : %s", err)
      }
        return  nil,err
   }
   list, err := result.Result()
   if err != nil {
         Logger.Printf("Result is wrong  : %s", err)
         return nil,err
   }
   return list, nil
}

func arc90(url ,location string) (*Article, error) {
  resp, err := http.Get(url)
  if err != nil{
    Logger.Println(err)
    return nil,err
  }
  defer resp.Body.Close()
  body, err := ioutil.ReadAll(resp.Body)

  doc, err := readability.NewDocument(string(body))
  if err != nil {
    Logger.Println(err)
    return  nil, err
  }
  /*create md  id*/
  fname := JoinName(url)
  fname = filepath.Join(location, fname )

 content := doc.Content()
 if len(content) < 114 {
       Logger.Printf("the content is short :%s", url)
       return nil,errors.New("invalid arc")
 }

 file , err := os.Create(fname)
 if err != nil {
    Logger.Println(err)
    return nil, err
 }
 defer file.Close()
file.WriteString(content)

 arc := Article {
        Id : fmt.Sprintf("%x" , md5.Sum( []byte(url) )),
        Url: url,
        Location: fname,
        Time: time.Now(),
        Title: doc.Title(),
}
 return &arc, nil
}
func CheckFileIsExist(filename string) (bool) {
      var exist = true
      if _, err := os.Stat(filename); os.IsNotExist(err) {
          exist = false
      }
      return exist
}

func JoinName(url string) string{
      md := fmt.Sprintf("%x" , md5.Sum( []byte(url) ))
      return  md  + ".html"
}

func SaveArc(arc Article) error{
  //Logger.Println(arc)
  jsob, err := json.Marshal(arc)
  multi, err := GRdb.Watch(BOARD_CUR_ARC_LIST)
  if  err != nil {
        Logger.Printf("Failed to watch db  : %s", err)
        return  err
   }
   defer multi.Close()

   result := multi.LLen(BOARD_CUR_ARC_LIST)
   len, err := result.Result()
   if err != nil {
         Logger.Printf("LLen is wrong  : %s", err)
         return err
   }
   var legacy *redis.StringCmd = nil
   id := fmt.Sprintf("%x" , md5.Sum( []byte(arc.Url) ))

   if len < 5 {
      ret, err:= multi.Exec( func () error {
           multi.RPush(BOARD_CUR_ARC_LIST,string(jsob))
           multi.HSet(BOARD_LEGACY_ARC_HASH, id, string(jsob))
           return nil
      })
      if  err != nil {
            Logger.Println(err,ret)
            return err
      }
       return nil

   }else {
     ret, err := multi.Exec( func () error {
          legacy =  multi.LPop(BOARD_CUR_ARC_LIST)
          multi.RPush(BOARD_CUR_ARC_LIST,string(jsob))
          multi.HSet(BOARD_LEGACY_ARC_HASH, id, string(jsob))
          return nil
     })
     if  err != nil {
           Logger.Println(err,ret)
           return err
     }
   }
   if legacy != nil {
     MoveCurToLegacy(legacy.Val())
   }
   return nil
}
func InitDb(){
  if GRdb == nil{
      GRdb =  redis.NewClient(&RedisOption)
  }
}
func MoveCurToLegacy (arts string) error {
      Logger.Println(arts)
      if len(arts) == 0 {
        return nil
      }
      var art Article
      err := json.Unmarshal([]byte(arts), &art)
      if err != nil{
        Logger.Printf("Failed to unMarshal : %s %s", arts, err)
        return err
      }

      md := fmt.Sprintf("%x" , md5.Sum( []byte(art.Url) ))
      key := BOARD_LEGACY_Week_ARC_HASH+art.Time.Weekday().String()

      if  length, err := GRdb.LLen(key).Result(); err != nil {
         return err
      }else{
        if length < 5 {
             GRdb.RPush(key, arts)
        }else {
            GRdb.LPop(key)
            GRdb.RPush(key, arts)
        }
      }

   return nil
}
