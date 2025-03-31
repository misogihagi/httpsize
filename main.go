package main

import (
        "io"
        "log"
        "net"
        "net/http"
        "net/http/httputil"
        "net/url"
)

const backendURL = "http://localhost:8000"

func main() {
        target, err := url.Parse(backendURL)
        if err != nil {
                log.Fatalf("ターゲットURLの解析に失敗: %v", err)
        }

        proxy := httputil.NewSingleHostReverseProxy(target)
        proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
                log.Printf("プロキシエラー: %v", err)

                // `context canceled` の場合、ログを分ける
                if err.Error() == "context canceled" {
                        log.Println("クライアントが接続を切断しました")
                        return
                }

                http.Error(w, "プロキシエラー", http.StatusBadGateway)
        }
        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                if isWebSocket(r) {
                        handleWebSocket(w, r, target)
                        return
                }
                proxy.ServeHTTP(w, r)
        })

        log.Println("HTTPSリバースプロキシを :4443 で起動中...")
        err = http.ListenAndServeTLS(":4443", "server.crt", "server.key", nil)
        if err != nil {
                log.Fatalf("HTTPSサーバーの起動に失敗: %v", err)
        }
}

func isWebSocket(r *http.Request) bool {
        return r.Header.Get("Upgrade") == "websocket" &&
                r.Header.Get("Connection") == "Upgrade"
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL) {
        dialer, err := net.Dial("tcp", target.Host)
        if err != nil {
                http.Error(w, "WebSocket接続失敗", http.StatusBadGateway)
                return
        }
        defer dialer.Close()

        hijacker, ok := w.(http.Hijacker)
        if !ok {
                http.Error(w, "WebSocketハンドリング失敗", http.StatusInternalServerError)
                return
        }

        clientConn, _, err := hijacker.Hijack()
        if err != nil {
                http.Error(w, "WebSocketハンドシェイク失敗", http.StatusInternalServerError)
                return
        }
        defer clientConn.Close()

        go io.Copy(dialer, clientConn)
        io.Copy(clientConn, dialer)
}
