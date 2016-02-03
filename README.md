Golang library for fast file transfers.  Not protocol compatible, but heavily inspired by the tsunami protocol:
* [How Tsunami works](http://tsunami-udp.cvs.sourceforge.net/viewvc/tsunami-udp/docs/howTsunamiWorks.txt)
* [C reference implementation](https://github.com/sebsto/tsunami-udp)

### Usage:

#### Client
  ```go
  config := gonami.NewConfig()
  e := gonami.BsonEncoder{}
  client := gonami.NewClient(downloadDir, config, e)

  client.GetFile(m["name"].(string), host)
  for p := range progress {
  	log.Println(p)
  }
```
```GetFile```  will return a channel that you can use to read the download progress on

#### Server
```go
  e := gonami.BsonEncoder{}
	s := gonami.NewServer(e, listenPort, uploadDirectory)
	go s.StartListening()
  ```
The ```Server``` struct contains a channel member that you can use to read the upload progress on
