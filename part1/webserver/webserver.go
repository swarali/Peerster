package webserver

import (
    "html/template"
    "net/http"
)

type Page struct {
    Name string
    Peers []string
    Messages []string
}

var IndexPage *Page
var WebServerSendChannel chan [2]string
var WebServerReceiveChannel chan [2]string

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
    //fmt.Println(r.Form)
    //msg := message.ClientMessage{}
    for k,v := range r.Form {
        //fmt.Println("client_msg:",k, v)
        WebServerReceiveChannel<-[2]string{k, v[0]}
    }
}

func StartWebServer(name string, peer_list []string,
                    webport string) {
    IndexPage = &Page{Name: name, Peers: peer_list}
    //fmt.Println("Starting Web Server")
    http.HandleFunc("/", index_handler)
    http.HandleFunc("/id", handler)
    http.HandleFunc("/message", handler)
    http.HandleFunc("/node", handler)
    go http.ListenAndServe(":"+webport, nil)

    for message_to_webclient := range WebServerSendChannel {
        operation := message_to_webclient[0]
        msg := message_to_webclient[1]
        if operation == "NewMessage" {
            IndexPage.Messages = append([]string{msg}, IndexPage.Messages...)
        } else if operation == "NewPeer" {
            IndexPage.Peers = append(IndexPage.Peers, msg)
        } else if operation == "NewID" {
            IndexPage.Name = msg
        }
    }
}

