package main

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elitah/fast-io"

	"github.com/astaxie/beego/httplib"
	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//go:embed static
var www_static embed.FS

type Request struct {
	sync.Mutex

	Key      string      `json:"key"`
	Name     string      `json:"name"`
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Header   http.Header `json:"header"`
	Deadline int64       `json:"deadline"`
}

func (this *Request) Timeout(unixnow int64, flags ...bool) bool {
	//
	this.Lock()
	defer this.Unlock()
	//
	if this.Deadline > unixnow {
		//
		if 0 < len(flags) && flags[0] {
			//
			this.Deadline = unixnow + 600
		}
		//
		return false
	}
	//
	return true
}

func main() {
	//
	const WINDOW_HEIGHT = 100
	const WINDOW_WIDTH = 200
	//
	var port int
	//
	var m sync.Map
	//
	var mw *walk.MainWindow
	//
	var inLE1, inLE2, inLE3 *walk.LineEdit
	//
	var btnCC *walk.PushButton
	//
	defer func() {
		//
		if err := recover(); nil != err {
			//
			buffer := make([]byte, 32*1024)
			//
			n := runtime.Stack(buffer, false)
			//
			buffer = buffer[:n]
			//
			if f, _err := os.OpenFile(
				fmt.Sprintf(
					"panic_%s.log",
					time.Now().UTC().Format("20060102_150405"),
				),
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				0644,
			); nil == _err {
				//
				fmt.Fprintf(f, "%v\n\n%s\n", err, buffer)
				//
				f.Close()
			} else {
				//
				fmt.Printf("%v\n\n%s\n", err, buffer)
			}
		}
	}()
	//
	if l, err := net.Listen("tcp4", ":0"); nil == err {
		//
		if address, ok := l.Addr().(*net.TCPAddr); ok {
			//
			port = address.Port
		}
		//
		go http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//
			switch r.URL.Path {
			case "/":
				//
				if data, err := www_static.ReadFile("static/index.html"); nil == err {
					//
					w.Header().Set("Content-Type", "text/html")
					//
					w.Write(data)
					//
					return
				}
			case "/list":
				//
				var list []interface{}
				//
				m.Range(func(key, value interface{}) bool {
					//
					if pr, ok := value.(*Request); ok {
						//
						list = append(list, pr)
					}
					//
					return true
				})
				//
				if data, err := json.Marshal(list); nil == err {
					//
					w.Header().Set("Content-Type", "application/json")
					//
					w.Write(data)
					//
					return
				}
			case "/download":
				//
				if "GET" == r.Method {
					//
					if key := r.FormValue("key"); "" != key {
						//
						if v, ok := m.Load(key); ok {
							//
							if pr, ok := v.(*Request); ok {
								//
								unixnow := time.Now().Unix()
								//
								if pr.Timeout(unixnow, true) {
									//
									break
								}
								//
								if req, err := http.NewRequest(pr.Method, pr.URL, nil); nil == err {
									//
									for key, values := range r.Header {
										//
										if "Host" == key {
											//
											continue
										}
										//
										for _, item := range values {
											//
											req.Header.Set(key, item)
										}
									}
									//
									for key, values := range pr.Header {
										//
										for _, item := range values {
											//
											req.Header.Set(key, item)
										}
									}
									//
									if resp, err := http.DefaultClient.Do(req); nil == err {
										//
										header := w.Header()
										//
										for key, values := range resp.Header {
											//
											for _, item := range values {
												//
												header.Set(key, item)
											}
										}
										//
										w.WriteHeader(resp.StatusCode)
										//
										fast_io.Copy(w, resp.Body)
										//
										return
									}
								}
							}
						}
					}
				}
			default:
				//
				if strings.HasPrefix(r.URL.Path, "/static/") {
					//
					if data, err := www_static.ReadFile(r.URL.Path[1:]); nil == err {
						//
						if ct := mime.TypeByExtension(filepath.Ext(r.URL.Path[1:])); "" != ct {
							//
							w.Header().Set("Content-Type", ct)
						}
						//
						w.Write(data)
						//
						return
					}
				}
			}
			//
			http.NotFound(w, r)
		}))
	}
	//
	go func() {
		//
		var unixnow int64
		//
		for {
			//
			unixnow = time.Now().Unix()
			//
			m.Range(func(key, value interface{}) bool {
				//
				if pr, ok := value.(*Request); ok {
					//
					if !pr.Timeout(unixnow) {
						//
						return true
					}
				}
				//
				m.Delete(key)
				//
				return true
			})
		}
	}()
	//
	declarative.MainWindow{
		AssignTo: &mw,
		Title:    "阿里云盘辅助工具",
		Layout:   declarative.VBox{},
		Children: []declarative.Widget{
			declarative.Label{Text: "请输入文件ID:"},
			declarative.LineEdit{AssignTo: &inLE1},
			declarative.Label{Text: "请输入解析服务地址（可选）:"},
			declarative.LineEdit{AssignTo: &inLE2},
			declarative.Label{Text: "请输入专有链接（可选）:"},
			declarative.LineEdit{AssignTo: &inLE3},
			declarative.PushButton{
				AssignTo: &btnCC,
				Text:     "获取下载地址",
				OnClicked: func() {
					//
					btnCC.SetEnabled(false)
					defer btnCC.SetEnabled(true)
					//
					reqFID := inLE1.Text()
					reqURL := inLE2.Text()
					reqLink := inLE3.Text()
					//
					if "" != reqLink {
						//
						if u, err := url.Parse(reqLink); nil == err {
							//
							if "aliyunpan" == u.Scheme && "" != u.Host {
								//
								if data, err := base64.StdEncoding.DecodeString(u.Host); nil == err {
									//
									result := struct {
										URLGet  string   `json:"urlget"`
										ID      string   `json:"id"`
										Origin  string   `json:"origin"`
										Origins []string `json:"origins"`
									}{}
									//
									if err := json.Unmarshal(data, &result); nil == err {
										//
										if "" != result.URLGet && "" != result.ID {
											//
											reqFID, reqURL = result.ID, result.URLGet
										}
									}
								}
							}
						}
					}
					//
					if "" == reqFID {
						//
						return
					}
					//
					if "" == reqURL {
						//
						return
					}
					//
					if resp, err := httplib.Get(reqURL).
						Param("fid", reqFID).
						Response(); nil == err {
						//
						r := &Request{
							Key:    reqFID,
							Header: make(http.Header),
						}
						//
						for key, values := range resp.Header {
							//
							if strings.HasPrefix(key, "Cdn-") {
								//
								switch key[4:] {
								case "Method":
									//
									r.Method = resp.Header.Get(key)
								case "Url":
									//
									r.URL = resp.Header.Get(key)
								case "Size":
								default:
									//
									for _, item := range values {
										//
										r.Header.Add(key[4:], item)
									}
								}
							}
						}
						//
						if "" != r.Method && "" != r.URL {
							//
							nowtime := time.Now()
							//
							r.Deadline = nowtime.Unix() + 600
							//
							m.Store(r.Key, r)
							//
							url := fmt.Sprintf("http://127.0.0.1:%d/download?key=%s", port, r.Key)
							//
							if err := walk.Clipboard().SetText(url); nil == err {
								//
								walk.MsgBox(
									mw,
									"提示",
									fmt.Sprintf(
										"下载地址已复制到剪贴板，请使用浏览器或下载工具进行下载，地址为：%s",
										url,
									),
									walk.MsgBoxIconInformation,
								)
							} else {
								//
								fmt.Println("Copy to Clipboard: ", err)
							}
						} else {
							//
							walk.MsgBox(
								mw,
								"提示",
								"无效的文件ID",
								walk.MsgBoxIconError,
							)
						}
					}
				},
			},
			declarative.PushButton{
				Text: "查看地址列表",
				OnClicked: func() {
					//
					if url := fmt.Sprintf("http://127.0.0.1:%d/", port); "" != url {
						//
						if err := walk.Clipboard().SetText(url); nil == err {
							//
							walk.MsgBox(
								mw,
								"提示",
								fmt.Sprintf(
									"已复制状态页地址到剪贴板，请使用浏览器打开，地址为：%s",
									url,
								),
								walk.MsgBoxIconInformation,
							)
						}
					}
				},
			},
		},
	}.Create()
	//
	mw.SetVisible(false)
	//
	walk.MsgBox(
		mw,
		"使用条款",
		"此工具仅用于交流学习之用途，使用此工具而产生的一切后果及连带责任与开发者无关，点击确定即代表接受此条款。",
		walk.MsgBoxIconInformation,
	)
	//
	win.SetWindowLong(
		mw.Handle(),
		win.GWL_STYLE,
		(win.GetWindowLong(mw.Handle(), win.GWL_STYLE)|win.WS_OVERLAPPED) & ^win.WS_MINIMIZEBOX & ^win.WS_MAXIMIZEBOX & ^win.WS_THICKFRAME,
	)
	//
	mw.SetBounds(walk.Rectangle{
		X:      int((win.GetSystemMetrics(win.SM_CXSCREEN) - WINDOW_WIDTH) / 2),
		Y:      int((win.GetSystemMetrics(win.SM_CYSCREEN) - WINDOW_HEIGHT) / 2),
		Width:  WINDOW_WIDTH,
		Height: WINDOW_HEIGHT,
	})
	//
	mw.SetVisible(true)
	//
	mw.Run()
}
