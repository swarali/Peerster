package webserver

import (
    "encoding/hex"
    "fmt"
    "html/template"
    "net/http"

    "github.com/Swarali/Peerster/message"
)

type Page struct {
    Name string
    Peers []string
    Messages []string
    PrivateMessages map[string][]string
    KnownRoutes []string
    UploadedFiles []string
    FoundFiles map[string]bool
}

var IndexPage *Page
var WebServerSendChannel chan message.ClientMessage
var WebServerReceiveChannel chan message.ClientMessage

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

func StartWebServer(name string, peer_list []string,
                    webport string) {
    IndexPage = &Page{Name: name, Peers: peer_list, PrivateMessages: make(map[string][]string), FoundFiles: make(map[string]bool)}
    //fmt.Println("Starting Web Server")
    http.HandleFunc("/", index_handler)
    http.HandleFunc("/id", handler)
    http.HandleFunc("/message", handler)
    http.HandleFunc("/node", handler)
    go http.ListenAndServe(":"+webport, nil)

    for message_to_webclient := range WebServerSendChannel {
        operation := message_to_webclient.Operation
        msg := message_to_webclient.Message
        if operation == "NewPeer" {
            IndexPage.Peers = append(IndexPage.Peers, msg)
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

