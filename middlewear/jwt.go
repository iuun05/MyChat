package middlewear

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var (
	TokenExpired = errors.New("Token is expired")
)

// jwt secret
var jwtSecret = []byte("ice_moss")

type Claims struct {
	UserID uint `json:"userId"`
	jwt.StandardClaims
}

// GenerateToken 根据用户的用户名和密码产生token
func GenerateToken(userId uint, iss string) (string, error) {
	// 为 token 设置有效时间
	nowtime := time.Now()
	expiredTime := nowtime.Add(48 * 30 * time.Hour)

	claims := Claims{
		UserID: userId,
		StandardClaims: jwt.StandardClaims{
			// 过期时间
			ExpiresAt: expiredTime.Unix(),
			// 发行人
			Issuer: iss,
		},
	}

	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	//该方法内部生成签名字符串，再用于获取完整、已签名的token
	token, err := tokenClaims.SignedString(jwtSecret)
	return token, err
}

// 鉴权
func JWY() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.PostForm("token")
		user := ctx.Query("userId")
		userId, err := strconv.Atoi(user)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, map[string]string{
				"message": "user id unauthorized",
			})
			// 立即终止后续的中间件和处理函数的执行。
			ctx.Abort()
			return
		}

		if token == "" {
			ctx.JSON(http.StatusUnauthorized, map[string]string{
				"message": "token us unauthorized",
			})
			ctx.Abort()
			return
		} else {
			claims, err := ParseToken(token)
			if err != nil {
				ctx.JSON(http.StatusUnauthorized, map[string]string{
					"message": "token lose efficacy",
				})
				ctx.Abort()
				return
			} else if time.Now().Unix() > claims.ExpiresAt {
				err = TokenExpired
				ctx.JSON(http.StatusUnauthorized, map[string]string{
					"message": "token is expired",
				})
				ctx.Abort()
				return
			}

			if claims.UserID != uint(userId) {
				ctx.JSON(http.StatusUnauthorized, map[string]string{
					"message": "Login is invalid",
				})
				ctx.Abort()
				return
			}
		}

		fmt.Println("login is valid, welcome !")
		// 继续执行后面的 handler
		ctx.Next()
	}
}

// ParseToken 根据传入的token值获取到Claims对象信息（进而获取其中的用户id）
func ParseToken(token string) (*Claims, error) {
	//用于解析鉴权的声明，方法内部主要是具体的解码和校验的过程，最终返回*Token
	tokenClaims, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if tokenClaims != nil {
		// 从tokenClaims中获取到Claims对象，并使用断言，将该对象转换为我们自己定义的Claims
		// 要传入指针，项目中结构体都是用指针传递，节省空间。
		if claims, ok := tokenClaims.Claims.(*Claims); ok && tokenClaims.Valid {
			return claims, nil
		}
	}
	return nil, err
}
