package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"
)

// 注册人机验证参数。
const (
	// 验证码有效期。
	captchaTTL = 2 * time.Minute
	// 校验通过后 ticket 的有效期，注册需要在这段时间内提交。
	captchaTicketTTL = 5 * time.Minute
	// 同一验证码允许的错误次数。
	captchaMaxAttempts = 5
	// 每 IP 每分钟生成/校验次数上限，避免被刷爆 CPU 和内存。
	captchaGeneratePerMinute = 30
	captchaVerifyPerMinute   = 20
	// 限流统计窗口。
	captchaRateWindow = time.Minute

	// 图片尺寸与字模参数。
	captchaCodeLen     = 4
	captchaImageWidth  = 160
	captchaImageHeight = 56
	captchaGlyphWidth  = 5
	captchaGlyphHeight = 7
	captchaGlyphScale  = 4
	captchaCharStep    = 34
	captchaNoiseDots   = 90
	captchaNoiseLines  = 3

	// 随机串长度（字节）。
	captchaIDBytes     = 12
	captchaTicketBytes = 32
)

// 去掉 0/O、1/I 等容易看错的字符，共 32 个，按字节取模不会产生偏差。
const captchaAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// 验证码相关错误码，前端据此提示并重新验证。
const (
	codeCaptchaInvalid     = "CAPTCHA_INVALID"
	codeCaptchaExpired     = "CAPTCHA_EXPIRED"
	codeCaptchaTooMany     = "CAPTCHA_TOO_MANY_ATTEMPTS"
	codeCaptchaRateLimited = "CAPTCHA_RATE_LIMITED"
)

// CaptchaChallenge 下发给前端的验证码题目，Image 为 data URL。
type CaptchaChallenge struct {
	ID        string `json:"id"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expires_in"`
}

// CaptchaTicket 人机验证通过后签发的一次性凭证。
type CaptchaTicket struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

// captchaError 带错误码的验证失败信息。
type captchaError struct {
	Code    string
	Message string
}

func (e *captchaError) Error() string {
	return e.Message
}

var (
	captchaBackground = color.RGBA{R: 0xf8, G: 0xf8, B: 0xf8, A: 0xff}
	// 深色系，保证和浅色背景有足够对比度。
	captchaCharColors = []color.RGBA{
		{R: 0x1d, G: 0x4e, B: 0x89, A: 0xff},
		{R: 0x7a, G: 0x2f, B: 0x8a, A: 0xff},
		{R: 0x0f, G: 0x6f, B: 0x5c, A: 0xff},
		{R: 0x9c, G: 0x3d, B: 0x1f, A: 0xff},
		{R: 0x33, G: 0x33, B: 0x33, A: 0xff},
	}
)

// captchaGlyphs 内置 5x7 点阵字模，只覆盖验证码字符集。
// 每行低 5 位有效，最高位是每行最左侧像素。
var captchaGlyphs = map[rune][captchaGlyphHeight]uint8{
	'2': {0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111},
	'3': {0b11111, 0b00010, 0b00100, 0b00010, 0b00001, 0b10001, 0b01110},
	'4': {0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	'5': {0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	'6': {0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	'8': {0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	'9': {0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
	'A': {0b01110, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001},
	'B': {0b11110, 0b10001, 0b10001, 0b11110, 0b10001, 0b10001, 0b11110},
	'C': {0b01110, 0b10001, 0b10000, 0b10000, 0b10000, 0b10001, 0b01110},
	'D': {0b11110, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b11110},
	'E': {0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b11111},
	'F': {0b11111, 0b10000, 0b10000, 0b11110, 0b10000, 0b10000, 0b10000},
	'G': {0b01110, 0b10001, 0b10000, 0b10111, 0b10001, 0b10001, 0b01111},
	'H': {0b10001, 0b10001, 0b10001, 0b11111, 0b10001, 0b10001, 0b10001},
	'J': {0b00111, 0b00010, 0b00010, 0b00010, 0b00010, 0b10010, 0b01100},
	'K': {0b10001, 0b10010, 0b10100, 0b11000, 0b10100, 0b10010, 0b10001},
	'L': {0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b10000, 0b11111},
	'M': {0b10001, 0b11011, 0b10101, 0b10101, 0b10001, 0b10001, 0b10001},
	'N': {0b10001, 0b11001, 0b10101, 0b10011, 0b10001, 0b10001, 0b10001},
	'P': {0b11110, 0b10001, 0b10001, 0b11110, 0b10000, 0b10000, 0b10000},
	'Q': {0b01110, 0b10001, 0b10001, 0b10001, 0b10101, 0b10010, 0b01101},
	'R': {0b11110, 0b10001, 0b10001, 0b11110, 0b10100, 0b10010, 0b10001},
	'S': {0b01111, 0b10000, 0b10000, 0b01110, 0b00001, 0b00001, 0b11110},
	'T': {0b11111, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100, 0b00100},
	'U': {0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01110},
	'V': {0b10001, 0b10001, 0b10001, 0b10001, 0b10001, 0b01010, 0b00100},
	'W': {0b10001, 0b10001, 0b10001, 0b10101, 0b10101, 0b11011, 0b10001},
	'X': {0b10001, 0b10001, 0b01010, 0b00100, 0b01010, 0b10001, 0b10001},
	'Y': {0b10001, 0b10001, 0b01010, 0b00100, 0b00100, 0b00100, 0b00100},
	'Z': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b10000, 0b11111},
}

// renderCaptchaImage 把验证码字符画成 PNG，答案不会以文本形式出现在响应里。
func renderCaptchaImage(code string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, captchaImageWidth, captchaImageHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: captchaBackground}, image.Point{}, draw.Src)

	// 噪点
	for i := 0; i < captchaNoiseDots; i++ {
		fillRect(img, mrand.IntN(captchaImageWidth), mrand.IntN(captchaImageHeight), 1, 1, captchaNoiseColor())
	}

	// 干扰线
	for i := 0; i < captchaNoiseLines; i++ {
		drawLine(
			img,
			mrand.IntN(captchaImageWidth), mrand.IntN(captchaImageHeight),
			mrand.IntN(captchaImageWidth), mrand.IntN(captchaImageHeight),
			captchaNoiseColor(),
		)
	}

	glyphWidth := captchaGlyphWidth * captchaGlyphScale
	startX := (captchaImageWidth - ((captchaCodeLen-1)*captchaCharStep + glyphWidth)) / 2
	startY := (captchaImageHeight - captchaGlyphHeight*captchaGlyphScale) / 2

	for i, ch := range code {
		glyph, ok := captchaGlyphs[ch]
		if !ok {
			continue
		}
		// 每个字符随机抖动，破坏固定网格。
		x := startX + i*captchaCharStep + mrand.IntN(7) - 3
		y := startY + mrand.IntN(9) - 4
		drawGlyph(img, glyph, x, y, captchaCharColors[mrand.IntN(len(captchaCharColors))])
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("编码验证码图片失败：%w", err)
	}

	return buf.Bytes(), nil
}

// captchaNoiseColor 随机生成浅色噪点，避免遮挡字符。
func captchaNoiseColor() color.RGBA {
	v := uint8(150 + mrand.IntN(70))
	return color.RGBA{R: v, G: v, B: v, A: 0xff}
}

// drawGlyph 按缩放倍数绘制点阵字模。
func drawGlyph(img *image.RGBA, glyph [captchaGlyphHeight]uint8, x, y int, clr color.RGBA) {
	for row := 0; row < captchaGlyphHeight; row++ {
		for col := 0; col < captchaGlyphWidth; col++ {
			if glyph[row]&(1<<(captchaGlyphWidth-1-col)) == 0 {
				continue
			}
			fillRect(
				img,
				x+col*captchaGlyphScale,
				y+row*captchaGlyphScale,
				captchaGlyphScale,
				captchaGlyphScale,
				clr,
			)
		}
	}
}

// fillRect 填充矩形，自动裁剪到图片范围内。
func fillRect(img *image.RGBA, x, y, w, h int, clr color.RGBA) {
	bounds := img.Bounds()
	for dy := 0; dy < h; dy++ {
		py := y + dy
		if py < bounds.Min.Y || py >= bounds.Max.Y {
			continue
		}
		for dx := 0; dx < w; dx++ {
			px := x + dx
			if px < bounds.Min.X || px >= bounds.Max.X {
				continue
			}
			img.SetRGBA(px, py, clr)
		}
	}
}

// drawLine 用 Bresenham 画 1px 直线。
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, clr color.RGBA) {
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	stepX := -1
	if x0 < x1 {
		stepX = 1
	}
	stepY := -1
	if y0 < y1 {
		stepY = 1
	}

	err := dx + dy
	for {
		fillRect(img, x0, y0, 1, 1, clr)
		if x0 == x1 && y0 == y1 {
			return
		}
		double := 2 * err
		if double >= dy {
			err += dy
			x0 += stepX
		}
		if double <= dx {
			err += dx
			y0 += stepY
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// captchaEntry 一道等待作答的验证码。
type captchaEntry struct {
	answer    string
	expiresAt time.Time
	attempts  int
}

// captchaTicketEntry 已通过验证的一次性凭证。
type captchaTicketEntry struct {
	expiresAt time.Time
	ip        string
}

// rateWindowState 单 IP 的时间窗口计数。
type rateWindowState struct {
	start time.Time
	count int
}

// CaptchaStore 验证码与 ticket 的内存存储。
// 站点是单实例 + SQLite，进程内存储足够；重启后前端重新验证即可。
type CaptchaStore struct {
	mu      sync.Mutex
	entries map[string]captchaEntry
	tickets map[string]captchaTicketEntry
	limits  map[string]rateWindowState
}

// NewCaptchaStore 创建空存储。
func NewCaptchaStore() *CaptchaStore {
	return &CaptchaStore{
		entries: make(map[string]captchaEntry),
		tickets: make(map[string]captchaTicketEntry),
		limits:  make(map[string]rateWindowState),
	}
}

// Create 生成一道新验证码。
func (s *CaptchaStore) Create(ip string) (CaptchaChallenge, error) {
	return s.create(time.Now(), ip)
}

func (s *CaptchaStore) create(now time.Time, ip string) (CaptchaChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked(now)
	if !s.allowLocked("generate|"+ip, captchaGeneratePerMinute, now) {
		return CaptchaChallenge{}, &captchaError{
			Code:    codeCaptchaRateLimited,
			Message: "请求过于频繁，请稍后再试",
		}
	}

	code, err := randomCaptchaCode()
	if err != nil {
		return CaptchaChallenge{}, err
	}
	imageBytes, err := renderCaptchaImage(code)
	if err != nil {
		return CaptchaChallenge{}, err
	}
	id, err := randomToken(captchaIDBytes)
	if err != nil {
		return CaptchaChallenge{}, err
	}

	s.entries[id] = captchaEntry{answer: code, expiresAt: now.Add(captchaTTL)}
	return CaptchaChallenge{
		ID:        id,
		Image:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageBytes),
		ExpiresIn: int(captchaTTL / time.Second),
	}, nil
}

// Verify 校验答案，通过后签发一次性 ticket。
func (s *CaptchaStore) Verify(ip, id, answer string) (CaptchaTicket, error) {
	return s.verify(time.Now(), ip, id, answer)
}

func (s *CaptchaStore) verify(now time.Time, ip, id, answer string) (CaptchaTicket, error) {
	id = strings.TrimSpace(id)
	answer = strings.ToUpper(strings.TrimSpace(answer))

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked(now)
	if !s.allowLocked("verify|"+ip, captchaVerifyPerMinute, now) {
		return CaptchaTicket{}, &captchaError{
			Code:    codeCaptchaRateLimited,
			Message: "请求过于频繁，请稍后再试",
		}
	}

	entry, ok := s.entries[id]
	if !ok || now.After(entry.expiresAt) {
		delete(s.entries, id)
		return CaptchaTicket{}, &captchaError{
			Code:    codeCaptchaExpired,
			Message: "验证码已过期，请刷新后重试",
		}
	}

	if entry.answer != answer {
		entry.attempts++
		if entry.attempts >= captchaMaxAttempts {
			delete(s.entries, id)
			return CaptchaTicket{}, &captchaError{
				Code:    codeCaptchaTooMany,
				Message: "验证码错误次数过多，请刷新后重试",
			}
		}
		s.entries[id] = entry
		return CaptchaTicket{}, &captchaError{
			Code:    codeCaptchaInvalid,
			Message: fmt.Sprintf("验证码错误，还可以再试 %d 次", captchaMaxAttempts-entry.attempts),
		}
	}

	// 一个验证码只能通过一次。
	delete(s.entries, id)
	ticket, err := randomToken(captchaTicketBytes)
	if err != nil {
		return CaptchaTicket{}, err
	}
	s.tickets[ticket] = captchaTicketEntry{expiresAt: now.Add(captchaTicketTTL), ip: ip}

	return CaptchaTicket{
		Ticket:    ticket,
		ExpiresIn: int(captchaTicketTTL / time.Second),
	}, nil
}

// ConsumeTicket 原子消费 ticket，并校验签发时的 IP。
func (s *CaptchaStore) ConsumeTicket(ip, ticket string) bool {
	return s.consumeTicket(time.Now(), ip, ticket)
}

func (s *CaptchaStore) consumeTicket(now time.Time, ip, ticket string) bool {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sweepLocked(now)
	entry, ok := s.tickets[ticket]
	if !ok {
		return false
	}
	delete(s.tickets, ticket)

	if now.After(entry.expiresAt) {
		return false
	}
	return entry.ip == ip
}

// peekAnswer 仅用于测试读取答案，生产代码不要调用。
func (s *CaptchaStore) peekAnswer(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	if !ok {
		return "", false
	}
	return entry.answer, true
}

// sweepLocked 惰性清理过期数据，避免长期运行内存只增不减。
func (s *CaptchaStore) sweepLocked(now time.Time) {
	for id, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, id)
		}
	}
	for ticket, entry := range s.tickets {
		if now.After(entry.expiresAt) {
			delete(s.tickets, ticket)
		}
	}
	for key, window := range s.limits {
		if now.Sub(window.start) >= captchaRateWindow {
			delete(s.limits, key)
		}
	}
}

// allowLocked 在时间窗口内计数，超过上限返回 false。
func (s *CaptchaStore) allowLocked(key string, limit int, now time.Time) bool {
	window, ok := s.limits[key]
	if !ok || now.Sub(window.start) >= captchaRateWindow {
		s.limits[key] = rateWindowState{start: now, count: 1}
		return true
	}
	if window.count >= limit {
		return false
	}
	window.count++
	s.limits[key] = window

	return true
}

// randomCaptchaCode 生成验证码字符；字符集长度是 32，按字节取模不会偏。
func randomCaptchaCode() (string, error) {
	raw, err := randomBytes(captchaCodeLen)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.Grow(captchaCodeLen)
	for _, b := range raw {
		builder.WriteByte(captchaAlphabet[int(b)%len(captchaAlphabet)])
	}

	return builder.String(), nil
}

// randomToken 生成 URL 安全的随机串。
func randomToken(size int) (string, error) {
	raw, err := randomBytes(size)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomBytes(size int) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("生成随机数失败：%w", err)
	}
	return buf, nil
}
