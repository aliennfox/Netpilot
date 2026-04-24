package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// OutboundToURI 把 sing-box outbound map 反向转成 URI, 供 QR 分享使用。
// 只支持 消费级主流协议; 其他返回 error 由调用方降级显示。
// 参考 nekobox fmt/{shadowsocks,trojan,v2ray,hysteria,tuic}/*Fmt.kt 的 toUri() 实现。
func OutboundToURI(ob map[string]interface{}) (string, error) {
	if ob == nil {
		return "", fmt.Errorf("outbound is nil")
	}
	typ, _ := ob["type"].(string)
	switch typ {
	case "shadowsocks":
		return outboundToSSURI(ob)
	case "trojan":
		return outboundToTrojanURI(ob)
	case "vmess":
		return outboundToVMessURI(ob)
	case "vless":
		return outboundToVLessURI(ob)
	case "hysteria2":
		return outboundToHysteria2URI(ob)
	case "tuic":
		return outboundToTuicURI(ob)
	default:
		return "", fmt.Errorf("%s 协议暂不支持导出 URI (仅支持 ss/trojan/vmess/vless/hysteria2/tuic)", typ)
	}
}

func obStr(ob map[string]interface{}, k string) string {
	if v, ok := ob[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func obInt(ob map[string]interface{}, k string) int {
	if v, ok := ob[k]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return 0
}

// obMap 提取嵌套对象 (如 tls/transport)
func obMap(ob map[string]interface{}, k string) map[string]interface{} {
	if v, ok := ob[k]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func outboundToSSURI(ob map[string]interface{}) (string, error) {
	method := obStr(ob, "method")
	password := obStr(ob, "password")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if method == "" || password == "" || server == "" || port == 0 {
		return "", fmt.Errorf("SS 缺少必要字段")
	}
	// sip002: ss://base64url(method:password)@host:port#name
	userInfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	u := fmt.Sprintf("ss://%s@%s:%d", userInfo, server, port)
	if tag != "" {
		u += "#" + url.PathEscape(tag)
	}
	return u, nil
}

func outboundToTrojanURI(ob map[string]interface{}) (string, error) {
	password := obStr(ob, "password")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if password == "" || server == "" || port == 0 {
		return "", fmt.Errorf("Trojan 缺少必要字段")
	}
	q := url.Values{}
	if tls := obMap(ob, "tls"); tls != nil {
		if sni, _ := tls["server_name"].(string); sni != "" {
			q.Set("sni", sni)
		}
		if alpn, ok := tls["alpn"].([]interface{}); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				q.Set("alpn", strings.Join(parts, ","))
			}
		}
	}
	if tr := obMap(ob, "transport"); tr != nil {
		if netType, _ := tr["type"].(string); netType != "" {
			q.Set("type", netType)
			if path, _ := tr["path"].(string); path != "" {
				q.Set("path", path)
			}
			if host, _ := tr["host"].(string); host != "" {
				q.Set("host", host)
			}
		}
	}
	u := fmt.Sprintf("trojan://%s@%s:%d", url.QueryEscape(password), server, port)
	if qs := q.Encode(); qs != "" {
		u += "?" + qs
	}
	if tag != "" {
		u += "#" + url.PathEscape(tag)
	}
	return u, nil
}

// outboundToVMessURI 输出 v2rayN 风格 base64 JSON vmess:// 链接 (消费级 App 基本都认)。
func outboundToVMessURI(ob map[string]interface{}) (string, error) {
	uuid := obStr(ob, "uuid")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if uuid == "" || server == "" || port == 0 {
		return "", fmt.Errorf("VMess 缺少必要字段")
	}
	j := map[string]interface{}{
		"v":    "2",
		"ps":   tag,
		"add":  server,
		"port": strconv.Itoa(port),
		"id":   uuid,
		"aid":  "0",
		"scy":  "auto",
		"net":  "tcp",
		"type": "none",
		"host": "",
		"path": "",
		"tls":  "",
	}
	if alterId := obInt(ob, "alter_id"); alterId > 0 {
		j["aid"] = strconv.Itoa(alterId)
	}
	if sec := obStr(ob, "security"); sec != "" {
		j["scy"] = sec
	}
	if tr := obMap(ob, "transport"); tr != nil {
		if netType, _ := tr["type"].(string); netType != "" {
			j["net"] = netType
			if path, _ := tr["path"].(string); path != "" {
				j["path"] = path
			}
			if host, _ := tr["host"].(string); host != "" {
				j["host"] = host
			}
			// ws headers.Host fallback
			if hdrs, _ := tr["headers"].(map[string]interface{}); hdrs != nil {
				if h, _ := hdrs["Host"].(string); h != "" && j["host"] == "" {
					j["host"] = h
				}
			}
		}
	}
	if tls := obMap(ob, "tls"); tls != nil {
		if en, _ := tls["enabled"].(bool); en {
			j["tls"] = "tls"
			if sni, _ := tls["server_name"].(string); sni != "" {
				j["sni"] = sni
			}
		}
	}
	raw, err := json.Marshal(j)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(raw), nil
}

func outboundToVLessURI(ob map[string]interface{}) (string, error) {
	uuid := obStr(ob, "uuid")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if uuid == "" || server == "" || port == 0 {
		return "", fmt.Errorf("VLESS 缺少必要字段")
	}
	q := url.Values{}
	q.Set("encryption", "none")
	if flow := obStr(ob, "flow"); flow != "" {
		q.Set("flow", flow)
	}
	if tls := obMap(ob, "tls"); tls != nil {
		if en, _ := tls["enabled"].(bool); en {
			q.Set("security", "tls")
			if sni, _ := tls["server_name"].(string); sni != "" {
				q.Set("sni", sni)
			}
			if reality := obMap(tls, "reality"); reality != nil {
				if en2, _ := reality["enabled"].(bool); en2 {
					q.Set("security", "reality")
					if pbk, _ := reality["public_key"].(string); pbk != "" {
						q.Set("pbk", pbk)
					}
					if sid, _ := reality["short_id"].(string); sid != "" {
						q.Set("sid", sid)
					}
				}
			}
			if utls := obMap(tls, "utls"); utls != nil {
				if fp, _ := utls["fingerprint"].(string); fp != "" {
					q.Set("fp", fp)
				}
			}
			if alpn, ok := tls["alpn"].([]interface{}); ok && len(alpn) > 0 {
				parts := make([]string, 0, len(alpn))
				for _, a := range alpn {
					if s, ok := a.(string); ok {
						parts = append(parts, s)
					}
				}
				if len(parts) > 0 {
					q.Set("alpn", strings.Join(parts, ","))
				}
			}
		}
	}
	if tr := obMap(ob, "transport"); tr != nil {
		if netType, _ := tr["type"].(string); netType != "" {
			q.Set("type", netType)
			if path, _ := tr["path"].(string); path != "" {
				q.Set("path", path)
			}
			if host, _ := tr["host"].(string); host != "" {
				q.Set("host", host)
			}
			if svcName, _ := tr["service_name"].(string); svcName != "" {
				q.Set("serviceName", svcName)
			}
		}
	}
	u := fmt.Sprintf("vless://%s@%s:%d", uuid, server, port)
	if qs := q.Encode(); qs != "" {
		u += "?" + qs
	}
	if tag != "" {
		u += "#" + url.PathEscape(tag)
	}
	return u, nil
}

func outboundToHysteria2URI(ob map[string]interface{}) (string, error) {
	password := obStr(ob, "password")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if password == "" || server == "" || port == 0 {
		return "", fmt.Errorf("Hysteria2 缺少必要字段")
	}
	q := url.Values{}
	if tls := obMap(ob, "tls"); tls != nil {
		if sni, _ := tls["server_name"].(string); sni != "" {
			q.Set("sni", sni)
		}
		if insecure, _ := tls["insecure"].(bool); insecure {
			q.Set("insecure", "1")
		}
	}
	if obfs := obMap(ob, "obfs"); obfs != nil {
		if typ, _ := obfs["type"].(string); typ != "" {
			q.Set("obfs", typ)
			if pw, _ := obfs["password"].(string); pw != "" {
				q.Set("obfs-password", pw)
			}
		}
	}
	u := fmt.Sprintf("hysteria2://%s@%s:%d", url.QueryEscape(password), server, port)
	if qs := q.Encode(); qs != "" {
		u += "?" + qs
	}
	if tag != "" {
		u += "#" + url.PathEscape(tag)
	}
	return u, nil
}

func outboundToTuicURI(ob map[string]interface{}) (string, error) {
	uuid := obStr(ob, "uuid")
	password := obStr(ob, "password")
	server := obStr(ob, "server")
	port := obInt(ob, "server_port")
	tag := obStr(ob, "tag")
	if uuid == "" || server == "" || port == 0 {
		return "", fmt.Errorf("TUIC 缺少必要字段 (uuid / server / port)")
	}
	q := url.Values{}
	if cc := obStr(ob, "congestion_control"); cc != "" {
		q.Set("congestion_control", cc)
	}
	if udp := obStr(ob, "udp_relay_mode"); udp != "" {
		q.Set("udp_relay_mode", udp)
	}
	if tls := obMap(ob, "tls"); tls != nil {
		if sni, _ := tls["server_name"].(string); sni != "" {
			q.Set("sni", sni)
		}
		if alpn, ok := tls["alpn"].([]interface{}); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s, ok := a.(string); ok {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				q.Set("alpn", strings.Join(parts, ","))
			}
		}
	}
	u := fmt.Sprintf("tuic://%s:%s@%s:%d", uuid, url.QueryEscape(password), server, port)
	if qs := q.Encode(); qs != "" {
		u += "?" + qs
	}
	if tag != "" {
		u += "#" + url.PathEscape(tag)
	}
	return u, nil
}
