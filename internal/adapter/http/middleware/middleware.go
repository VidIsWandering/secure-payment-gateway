package middleware

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"secure-payment-gateway/internal/core/ports"
	"secure-payment-gateway/pkg/apperror"
	"secure-payment-gateway/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	// Header names for HMAC authentication
	HeaderAccessKey = "X-Merchant-Access-Key"
	HeaderSignature = "X-Signature"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"

	// Max timestamp drift allowed (60 seconds)
	maxTimestampDrift = 60 * time.Second

	// Nonce TTL (120 seconds)
	nonceTTL = 120 * time.Second

	// Context keys
	CtxMerchantID  = "merchant_id"
	CtxAccessKey   = "access_key"
	CtxMerchantKey = "merchant"
)

// HMACAuth creates a middleware that verifies HMAC-SHA256 signatures.
// Pipeline: Check timestamp -> Check nonce -> Verify signature.
func HMACAuth(
	merchantRepo ports.MerchantRepository,
	encSvc ports.EncryptionService,
	sigSvc ports.SignatureService,
	nonceStore ports.NonceStore,
	log zerolog.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessKey := c.GetHeader(HeaderAccessKey)
		signature := c.GetHeader(HeaderSignature)
		timestampStr := c.GetHeader(HeaderTimestamp)
		nonce := c.GetHeader(HeaderNonce)

		if accessKey == "" || signature == "" || timestampStr == "" || nonce == "" {
			response.Error(c, apperror.ErrInvalidAccessKey())
			c.Abort()
			return
		}

		// Step 1: Timestamp check
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			response.Error(c, apperror.ErrTimestampExpired())
			c.Abort()
			return
		}
		now := time.Now().Unix()
		if math.Abs(float64(now-timestamp)) > maxTimestampDrift.Seconds() {
			response.Error(c, apperror.ErrTimestampExpired())
			c.Abort()
			return
		}

		// Step 2: Lookup merchant and check nonce
		merchant, err := merchantRepo.GetByAccessKey(c.Request.Context(), accessKey)
		if err != nil {
			log.Error().Err(err).Msg("failed to fetch merchant")
			response.Error(c, apperror.InternalError(err))
			c.Abort()
			return
		}
		if merchant == nil {
			response.Error(c, apperror.ErrInvalidAccessKey())
			c.Abort()
			return
		}
		if !merchant.IsActive() {
			response.Error(c, apperror.ErrMerchantSuspended())
			c.Abort()
			return
		}

		isNew, err := nonceStore.CheckAndSet(c.Request.Context(), merchant.ID.String(), nonce, nonceTTL)
		if err != nil {
			log.Warn().Err(err).Msg("nonce store error, allowing request")
		} else if !isNew {
			response.Error(c, apperror.ErrNonceUsed())
			c.Abort()
			return
		}

		// Step 3: Signature verification
		secretKey, err := encSvc.Decrypt(merchant.SecretKeyEnc)
		if err != nil {
			log.Error().Err(err).Msg("failed to decrypt merchant secret key")
			response.Error(c, apperror.InternalError(err))
			c.Abort()
			return
		}

		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			response.Error(c, apperror.Validation("cannot read request body"))
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		canonical := sigSvc.BuildCanonicalString(
			c.Request.Method,
			c.Request.URL.Path,
			timestamp,
			nonce,
			string(bodyBytes),
		)

		if !sigSvc.Verify(secretKey, canonical, signature) {
			response.Error(c, apperror.ErrInvalidSignature())
			c.Abort()
			return
		}

		c.Set(CtxMerchantID, merchant.ID)
		c.Set(CtxAccessKey, merchant.AccessKey)
		c.Set(CtxMerchantKey, merchant)

		c.Next()
	}
}

// JWTAuth creates a middleware that validates JWT tokens for dashboard routes.
func JWTAuth(tokenSvc ports.TokenService, log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			response.Error(c, apperror.ErrInvalidToken())
			c.Abort()
			return
		}

		tokenStr := authHeader[7:]
		claims, err := tokenSvc.Validate(tokenStr)
		if err != nil {
			response.Error(c, apperror.ErrInvalidToken())
			c.Abort()
			return
		}

		c.Set(CtxMerchantID, claims.MerchantID)
		c.Set(CtxAccessKey, claims.AccessKey)
		c.Next()
	}
}

// RequestLogger creates a middleware that logs every HTTP request.
func RequestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()

		event := log.Info()
		if status >= http.StatusInternalServerError {
			event = log.Error()
		} else if status >= http.StatusBadRequest {
			event = log.Warn()
		}

		event.
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", status).
			Dur("latency", latency).
			Str("client_ip", c.ClientIP()).
			Msg("http request")
	}
}

// Recovery creates a panic recovery middleware.
func Recovery(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Str("path", c.Request.URL.Path).Msg("panic recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error_code": "SYS_001",
					"message":    "Internal server error",
				})
			}
		}()
		c.Next()
	}
}
