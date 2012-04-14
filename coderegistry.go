package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"
)

var messageCodes = make(map[int]*MessageCode)

type MessageCode struct {
	Code        int
	Deleted     bool
	Title       string
	Description string
	Created     time.Time
	CreatedIp   net.IP
	Modified    time.Time
	ModifiedIp  net.IP
}

var gobFilename = "codes.gob"

//

func main() {
	gob.Register(messageCodes)
	file, err := os.Open(gobFilename)
	if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOENT {
		fmt.Println("No existing file.  We'll just start from scratch.")
	} else if err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		defer file.Close()
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(&messageCodes)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/modify", modifyHandler)
	fmt.Println(http.ListenAndServeTLS(":13579", "ssl.crt", "ssl.key", nil))
	os.Exit(1)
}

//

func outputHeader(w http.ResponseWriter, title string) {
	fmt.Fprintf(w, "<html>\n<head>\n<title>%s • Bumble Message Code Registry</title>\n", title)
	fmt.Fprintf(w, "<style>\n.submit {width:80px}\n.deleted * {color:#CCC}\n.deleted .undelete {color:#000}\n.id {}\n.title {font-weight:bold}\n.title input {font-weight:bold;width:300px}\n.description {}\n.description input {width:300px}\n</style>\n")
	fmt.Fprintf(w, "</head>\n<body><h1>%s</h1>\n", title)
}

func outputFooter(w http.ResponseWriter) {
	fmt.Fprintln(w, "</body>\n</html>")
}

//

var modlock = make(chan bool, 1)

func addCode(title string, description string, ip net.IP) *MessageCode {
	modlock <- true
	code := new(MessageCode)
	code.Code = len(messageCodes)
	code.Deleted = false
	code.Title = title
	code.Description = description
	code.Created = time.Now().UTC()
	code.Modified = code.Created
	code.CreatedIp = ip
	code.ModifiedIp = code.CreatedIp
	messageCodes[code.Code] = code
	file, err := os.Create("codes.gob")
	if err != nil {
		<-modlock
		return nil
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(messageCodes)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	<-modlock
	return code
}

func modifyCode(id int, title string, description string, ip net.IP) *MessageCode {
	modlock <- true
	code := messageCodes[id]
	code.Title = title
	code.Description = description
	code.Modified = time.Now().UTC()
	code.ModifiedIp = ip
	messageCodes[code.Code] = code
	file, err := os.Create("codes.gob")
	if err != nil {
		<-modlock
		return nil
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(messageCodes)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	<-modlock
	return code
}

func deleteCode(id int, deleted bool, ip net.IP) *MessageCode {
	modlock <- true
	code := messageCodes[id]
	code.Deleted = deleted
	messageCodes[code.Code] = code
	file, err := os.Create("codes.gob")
	if err != nil {
		<-modlock
		return nil
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(messageCodes)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	<-modlock
	return code
}

//

func indexHandler(w http.ResponseWriter, r *http.Request) {
	outputHeader(w, "Message Code List")
	defer outputFooter(w)
	fmt.Fprintln(w, "<table>")
	fmt.Fprintln(w, "<tr><th>ID</th><th>Title</th><th>Description</th><th>Actions</th></tr>")
	fmt.Fprintf(w, "<form method='post' action='add'><tr class='add'><td class='id'>NEW</td><td class='title'><input id='startfocus' name='title' /></td><td class='description'><input name='description' /></td><td><input class='submit' type='submit' name='save' value='Save' /></td></tr></form>\n<script>document.getElementById('startfocus').focus()</script>\n")
	for i := 0; i < len(messageCodes); i++ {
		code := messageCodes[i]
		class := ""
		deleteName := "delete"
		deleteValue := "Delete"
		if code.Deleted {
			class = "deleted"
			deleteName = "undelete"
			deleteValue = "Undelete"
		}
		fmt.Fprintf(w, "<form method='post' action='modify'><input type='hidden' name='id' value='%d' /><tr class='%s' title='Modified :%s via %s\nCreated :%s via %s'><td class='id'>%d</td><td class='title'><input name='title' value='%s' /></td><td class='description'><input name='description' value='%s' /></td><td><input class='submit' type='submit' name='save' value='Save' /> <input class='submit %s' type='submit' name='%s' value='%s' /></td></tr></form>\n", code.Code, class, code.Modified, code.ModifiedIp, code.Created, code.CreatedIp, code.Code, code.Title, code.Description, deleteName, deleteName, deleteValue)
	}
	fmt.Fprintln(w, "</table>")
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	if title == "" {
		fmt.Fprintln(w, "Requires TITLE field.")
		return
	}
	description := r.FormValue("description")
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	addCode(title, description, net.ParseIP(host))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func modifyHandler(w http.ResponseWriter, r *http.Request) {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	idStr := r.FormValue("id")
	if idStr == "" {
		fmt.Fprintln(w, "Requires ID field.")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 0)
	if err != nil {
		fmt.Fprintln(w, "Requires ID field that is an integer.")
		return
	}
	deleteStr := r.FormValue("delete")
	delete := deleteStr != ""
	undeleteStr := r.FormValue("undelete")
	undelete := undeleteStr != ""
	if delete || undelete {
		deleteCode(int(id), delete, net.ParseIP(host))
	}
	title := r.FormValue("title")
	if title == "" {
		fmt.Fprintln(w, "Requires TITLE field.")
		return
	}
	description := r.FormValue("description")
	modifyCode(int(id), title, description, net.ParseIP(host))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
