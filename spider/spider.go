package main
import(
  "gopkg.in/redis.v3"
  "github.com/PuerkitoBio/goquery"
  "log"
  "os"
  "regexp"
  "runtime"
  "fmt"
  "crypto/md5"
)

var (
   RedisOption =  redis.Options {
      Addr: "127.0.0.1:6379",
      Password:"",
      DB: 0,
  }
  GRdb  * redis.Client = nil
  SPIDER_ROOTURL_KEY = "SPIDER:ROOTURL"
  SPIDER_URL_REG_KEY = "SPIDER:URLREGEXP"
  BOARD_LEGACY_ARC_HASH = "IBORAD:LEGACYARTICLE"
  WHALE_URL_LIST =        "WHALE:URL"

 	Logger *log.Logger = nil
  RegexpUrl  *regexp.Regexp = nil   // regexp.MustCompile(`http://blog.sina.com.cn/s/blog_[A-z-0-9]*.html$`)
)

func main(){
  var MULTICORE int = runtime.NumCPU() //number of core
        runtime.GOMAXPROCS(MULTICORE) //running in multicore
  Logfile ,_ := os.OpenFile("/var/iboard/log/iboard.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE,660)
  Logger = log.New(Logfile, "[spider]", log.LstdFlags) //os.Stdout
  defer Logfile.Close()


  var newurl = make([]string,0)
   rooturl, err := GetSpiderMap()
   if  err != nil{
     Logger.Fatal(err)
   }

   urlreg, err := GetRegexpMap()
   if  err != nil{
     Logger.Fatal(err)
   }

   for k, v  := range rooturl {
      if  reg, ok := urlreg[k]; ok {
          RegexpUrl  = regexp.MustCompile(reg)
          newurl = ExtractNewUrl(v, RegexpUrl)
      }else{
          Logger.Printf("failed to find regexp for %s %s",k, v)
      }
   }

   //Logger.Println(newurl)

   if len(newurl) != 0 {
       SaveURL(newurl)
  }

}//over main

func  GetRegexpMap() (map[string]string, error){
  if (GRdb == nil){
      GRdb =  redis.NewClient(&RedisOption)
  }
  result := GRdb.HGetAllMap(SPIDER_URL_REG_KEY)
  if err := result.Err(); err != nil {
        Logger.Printf("Failed to get app from db  : %s", err)
         return  nil,err
   }
   rmap, err := result.Result()
   if err != nil {
         Logger.Printf("Result is wrong  : %s", err)
         return nil,err
   }
   return rmap, nil
}

func  GetSpiderMap() (map[string]string, error){
  if (GRdb == nil){
      GRdb =  redis.NewClient(&RedisOption)
  }
   result := GRdb.HGetAllMap(SPIDER_ROOTURL_KEY)
  if err := result.Err(); err != nil {
        Logger.Printf("Failed to get app from db  : %s", err)
         return  nil,err
   }
   rmap, err := result.Result()
   if err != nil {
         Logger.Printf("Result is wrong  : %s", err)
         return nil,err
   }
   return rmap, nil
}
func ExtractNewUrl(rooturl string, rex * regexp.Regexp)([]string){
 var   surl = make([]string, 0)
         murl := make(map[string] bool)
  doc, err := goquery.NewDocument(rooturl)
	if err != nil {
		Logger.Println("Unable to create document", err)
		return make([]string,0)
	}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
    if attr , is := s.Attr("href"); is {
        if rex.MatchString(attr){
             if !murl[attr] {
                   murl[attr] = true
             }
        }
 		}
	})

 if  len(murl) != 0 {
     for k, _  := range murl {
         surl = append(surl,  k)
     }
 }
  return  surl
}
func SaveURL( urllist []string) error{
  if (GRdb == nil){
      GRdb =  redis.NewClient(&RedisOption)
  }
  for _, v  := range urllist {
    md := fmt.Sprintf("%x" , md5.Sum( []byte(v) ))
    is, err := GRdb.HExists(BOARD_LEGACY_ARC_HASH,md).Result()
    if err == nil && is {
        //Logger.Printf(" URL : %s is exist",v )
        continue
    }
    result := GRdb.RPush(WHALE_URL_LIST,v)
    if err := result.Err(); err != nil {
          Logger.Printf("Failed to save URL to db  : %s", err)
           return  err
     }
  }
   return nil
}
