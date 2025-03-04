package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"github.com/huoxue1/study_xxqg/lib"
	"github.com/huoxue1/study_xxqg/model"
	"github.com/huoxue1/study_xxqg/utils"
)

var (
	state = sync.Map{}
)

func CheckToken() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.Param("token")
		config := lib.GetConfig()
		md5 := utils.StrMd5(config.Web.Account + config.Web.Password)
		if md5 == token {
			ctx.JSON(200, Resp{
				Code:    200,
				Message: "",
				Data:    nil,
				Success: true,
				Error:   "",
			})
		} else {
			ctx.JSON(200, Resp{
				Code:    403,
				Message: "",
				Data:    nil,
				Success: false,
				Error:   "",
			})
		}
	}
}

func Login() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		type user struct {
			Account  string `json:"account"`
			Password string `json:"password"`
		}
		u := new(user)
		_ = ctx.BindJSON(u)
		config := lib.GetConfig()
		if u.Account == config.Web.Account && u.Password == config.Web.Password {
			ctx.JSON(200, Resp{
				Code:    200,
				Message: "登录成功",
				Data:    utils.StrMd5(u.Account + u.Password),
				Success: true,
				Error:   "",
			})
		} else {
			ctx.JSON(200, Resp{
				Code:    403,
				Message: "登录失败，请联系管理员",
				Data:    "",
				Success: false,
				Error:   "",
			})
		}
	}
}

func getScore() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.Query("token")
		score, err := lib.GetUserScore(model.TokenToCookies(token))
		if err != nil {
			ctx.JSON(403, Resp{
				Code:    403,
				Message: "",
				Data:    err.Error(),
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		ctx.JSON(200, Resp{
			Code:    200,
			Message: "获取成功",
			Data:    lib.PrintScore(score),
			Success: true,
			Error:   "",
		})
	}
}

func addUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		type params struct {
			Code  string `json:"code"`
			State string `json:"state"`
		}
		p := new(params)
		err := ctx.BindJSON(p)
		if err != nil {
			ctx.JSON(403, Resp{
				Code:    403,
				Message: "",
				Data:    err.Error(),
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		_, err = lib.GetToken(p.Code, p.State)
		if err != nil {
			ctx.JSON(403, Resp{
				Code:    403,
				Message: "",
				Data:    err.Error(),
				Success: false,
				Error:   err.Error(),
			})
			return
		}
		ctx.JSON(200, Resp{
			Code:    200,
			Message: "登录成功",
			Data:    "登录成功",
			Success: true,
			Error:   "",
		})
	}
}

func getUsers() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		users, err := model.Query()
		if err != nil {
			return
		}
		if users == nil {
			users = []*model.User{}
		}
		var datas []map[string]interface{}
		for _, user := range users {
			var isStudy = false
			_, ok := state.Load(user.UID)
			if ok {
				isStudy = true
			}
			datas = append(datas, map[string]interface{}{
				"nick":       user.Nick,
				"uid":        user.UID,
				"token":      user.Token,
				"login_time": user.LoginTime,
				"is_study":   isStudy,
			})
		}
		ctx.JSON(200, Resp{
			Code:    200,
			Message: "查询成功",
			Data:    datas,
			Success: true,
			Error:   "",
		})
	}
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		origin := c.Request.Header.Get("Origin") // 请求头部
		if origin != "" {
			// 接收客户端发送的origin （重要！）
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			// 服务器支持的所有跨域请求的方法
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE,UPDATE")
			// 允许跨域设置可以返回其他子段，可以自定义字段
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Length, X-CSRF-Token, Token,session")
			// 允许浏览器（客户端）可以解析的头部 （重要）
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers")
			// 设置缓存时间
			c.Header("Access-Control-Max-Age", "172800")
			// 允许客户端传递校验信息比如 cookie (重要)
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 允许类型校验
		if method == "OPTIONS" {
			c.JSON(http.StatusOK, "ok!")
		}

		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic info is: %v", err)
			}
		}()

		c.Next()
	}
}

func study() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uid := ctx.Query("uid")
		user := model.Find(uid)
		core := &lib.Core{
			ShowBrowser: false,
			Push: func(kind, message string) {
			},
		}
		core.Init()
		state.Store(uid, core)
		go func() {
			core.LearnArticle(user)
			core.LearnVideo(user)
			core.RespondDaily(user, "daily")
			core.RespondDaily(user, "weekly")
			core.RespondDaily(user, "special")
			state.Delete(uid)
		}()
	}
}

func stopStudy() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uid := ctx.Query("uid")
		value, ok := state.Load(uid)
		if !ok {
			return
		}
		core := value.(*lib.Core)
		core.Quit()
		ctx.JSON(200, Resp{
			Code:    200,
			Message: "",
			Data:    "",
			Success: true,
			Error:   "",
		})
	}
}

func getLog() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.File(fmt.Sprintf("./logs/%v.log", time.Now().Format("2006-01-02")))
	}
}

func sign() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		response, err := http.Get("https://pc-api.xuexi.cn/open/api/sns/sign") //nolint:bodyclose
		if err != nil {
			return
		}

		ctx.Writer.WriteHeader(200)
		data, _ := io.ReadAll(response.Body)
		ctx.Writer.Write(data)
	}
}

func generate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		remote, _ := url.Parse("https://login.xuexi.cn/")
		proxy := httputil.NewSingleHostReverseProxy(remote)
		proxy.Director = func(req *http.Request) {
			req.Header = ctx.Request.Header
			req.Host = remote.Host
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.Path = ctx.Param("proxyPath")
		}
		proxy.ServeHTTP(ctx.Writer, ctx.Request)
	}
}
