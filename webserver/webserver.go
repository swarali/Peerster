package webserver

import (
    "encoding/hex"
    "fmt"
    "html/template"
    "net/http"

    "github.com/Swarali/Peerster/message"
	"os"
	"encoding/json"
	"log"
	"strconv"
)

type Page struct {
    Name string
    Peers []string
    TrustedPeers []string
    Messages []string
    PrivateMessages map[string][]string
    KnownRoutes []string
    UploadedFiles []string
    FoundFiles map[string]bool
}

type FileMeta struct {
	Size int64
	RealSize int64
	Exists bool
}

var IndexPage *Page
var WebServerSendChannel chan message.ClientMessage
var WebServerReceiveChannel chan message.ClientMessage
var SHARING_DIR string

func index_handler(w http.ResponseWriter, r *http.Request) {
    //title := "Welcome to Peerster"
    //fmt.Fprintf(w, "<h1>%s</h1><div>%s</div>", page.Name,
                   //strings.Join(page.Peers, ","))
    //fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
    //request := make(chan bool)
    //GetChannel<-request
    //<-request
    t, _ := template.ParseFiles("webserver/index.html")
    t.Execute(w, IndexPage)
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	var exists bool
	var file_size int64
	var real_size int64

	r.ParseForm()

	file_name := r.Form["filename"][0]
	file_path := SHARING_DIR + "/" + file_name
	size_path := SHARING_DIR + "/size_" + file_name

	if _, err := os.Stat(file_path); os.IsNotExist(err) {
		exists = false
		file_size = 0
	} else {
		exists = true

		file, open_err := os.Open(file_path)
		file_info, stat_err := file.Stat()
		if open_err!= nil || stat_err != nil {
			fmt.Println("Error while accessing", file_path, open_err, stat_err)
			log.Println("Error while accessing", file_path, open_err, stat_err)
			return
		}

		file_size = file_info.Size()

		size_file, open_err := os.Open(size_path)
		size_info, stat_err := size_file.Stat()
		if open_err!= nil || stat_err != nil {
			fmt.Println("Error while accessing", size_path, open_err, stat_err)
			log.Println("Error while accessing", size_path, open_err, stat_err)

			real_size = int64(0)
		} else {
			size_file_size := size_info.Size()

			data := make([]byte, size_file_size)

			size_file.Read(data)

			varr, _ := strconv.Atoi(string(data))

			real_size = int64(varr)
		}
	}

	info := make(map[string]FileMeta)

	info[file_name] = FileMeta{ Size: file_size, Exists: exists, RealSize: real_size }

	json.NewEncoder(w).Encode(info)
}

func handler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    fmt.Println(r.Form)
    msg := message.ClientMessage{}
    for k,v := range r.Form {
        if k == "Operation" {
            msg.Operation = v[0]
        } else if k == "Message" {
            msg.Message = v[0]
        } else if k == "Destination" {
            msg.Destination = v[0]
        } else if k == "Hash" {
            msg.HashValue, _ = hex.DecodeString(v[0])
        }
    }
    WebServerReceiveChannel<-msg
}

func StartWebServer(name string, peer_list []string, webport string, sharing_dir string) {
    SHARING_DIR = sharing_dir

	IndexPage = &Page{Name: name, Peers: peer_list, TrustedPeers: peer_list, PrivateMessages: make(map[string][]string), FoundFiles: make(map[string]bool)}
    //fmt.Println("Starting Web Server")
    http.HandleFunc("/", index_handler)
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    http.Handle("/streaming/", http.StripPrefix("/streaming/", http.FileServer(http.Dir(sharing_dir))))
    http.HandleFunc("/id", handler)
    http.HandleFunc("/message", handler)
    http.HandleFunc("/node", handler)
    http.HandleFunc("/file", fileHandler)
    go http.ListenAndServe(":"+webport, nil)

    for message_to_webclient := range WebServerSendChannel {
        operation := message_to_webclient.Operation
        msg := message_to_webclient.Message
        if operation == "NewPeer" {
            IndexPage.Peers = append(IndexPage.Peers, msg)
        } else if operation == "NewTrustedPeer" {
            IndexPage.TrustedPeers = append(IndexPage.TrustedPeers, msg)
        } else if operation == "NewID" {
            IndexPage.Name = msg
        } else if operation == "NewMessage" {
            origin := message_to_webclient.Origin
            IndexPage.Messages = append([]string{origin+":"+msg},
                                        IndexPage.Messages...)
        } else if operation == "NewPrivateMessage" {
            if message_to_webclient.Origin != "" {
                origin := message_to_webclient.Origin
                IndexPage.PrivateMessages[origin] = append([]string{origin+":"+msg}, IndexPage.PrivateMessages[origin]...)
            } else if message_to_webclient.Destination != "" {
                destination := message_to_webclient.Destination
                IndexPage.PrivateMessages[destination] = append([]string{IndexPage.Name+":"+msg}, IndexPage.PrivateMessages[destination]...)
            }
        } else if operation == "NewRoute" {
            IndexPage.KnownRoutes = append(IndexPage.KnownRoutes, msg)
        } else if operation == "NewFileUpload" {
            IndexPage.UploadedFiles = append(IndexPage.UploadedFiles, msg)
        } else if operation == "NewFileReply" {
            IndexPage.FoundFiles[msg] = true
        }
    }
}

