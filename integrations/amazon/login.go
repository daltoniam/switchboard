package amazon

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TOTP requires SHA-1 per RFC 6238
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	signinURL = "https://www.%s/ap/signin?openid.pape.max_auth_age=0&openid.return_to=https%%3A%%2F%%2Fwww.%s%%2F&openid.identity=http%%3A%%2F%%2Fspecs.openid.net%%2Fauth%%2F2.0%%2Fidentifier_select&openid.assoc_handle=usflex&openid.mode=checkid_setup&openid.claimed_id=http%%3A%%2F%%2Fspecs.openid.net%%2Fauth%%2F2.0%%2Fidentifier_select&openid.ns=http%%3A%%2F%%2Fspecs.openid.net%%2Fauth%%2F2.0"

	loginTimeout     = 30 * time.Second
	loginPollWait    = 2 * time.Second
	otpFieldSelector = `input[name="otpCode"], input#auth-mfa-otpcode`
	otpSubmitBtn     = `input#auth-signin-button, #auth-signin-button-announce`
)

// login performs a full sign-in flow using Playwright:
//
//	email page → password page → optional OTP page.
//
// Requires a.browserSvc to be non-nil. On success the session cookies
// are stored in the browser context and remain valid for subsequent fetches.
// The entire flow is bounded by loginTimeout to prevent hanging.
func (a *amazon) login(ctx context.Context) error {
	if a.email == "" || a.password == "" {
		return fmt.Errorf("amazon: email and password are required for auto-login")
	}

	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	sess, err := a.ensureSession(ctx)
	if err != nil {
		return err
	}

	pg, err := sess.NewPage(ctx)
	if err != nil {
		return fmt.Errorf("amazon: new page for login: %w", err)
	}
	defer pg.Close() //nolint:errcheck

	url := fmt.Sprintf(signinURL, a.domain, a.domain)
	if err := pg.Navigate(ctx, url); err != nil {
		return fmt.Errorf("amazon: navigate sign-in: %w", err)
	}

	// Step 1: email
	if err := pg.WaitForSelector(ctx, "#ap_email"); err != nil {
		return fmt.Errorf("amazon: email field not found: %w", err)
	}
	if err := pg.Fill(ctx, "#ap_email", a.email); err != nil {
		return fmt.Errorf("amazon: fill email: %w", err)
	}
	if err := pg.Click(ctx, "#continue"); err != nil {
		return fmt.Errorf("amazon: click continue: %w", err)
	}

	// Step 2: password
	if err := ctxSleep(ctx, loginPollWait); err != nil {
		return err
	}
	if err := pg.WaitForSelector(ctx, "#ap_password"); err != nil {
		return fmt.Errorf("amazon: password field not found (CAPTCHA or unusual page?): %w", err)
	}
	if err := pg.Fill(ctx, "#ap_password", a.password); err != nil {
		return fmt.Errorf("amazon: fill password: %w", err)
	}
	if err := pg.Click(ctx, "#signInSubmit"); err != nil {
		return fmt.Errorf("amazon: click sign-in: %w", err)
	}

	// Step 3: check for OTP / MFA prompt
	if err := ctxSleep(ctx, loginPollWait); err != nil {
		return err
	}
	html, err := pg.Content(ctx)
	if err != nil {
		return fmt.Errorf("amazon: read post-login page: %w", err)
	}

	if strings.Contains(html, "otpCode") || strings.Contains(html, "auth-mfa-otpcode") {
		if a.otpSecret == "" {
			return fmt.Errorf("amazon: MFA/OTP required but otp_secret not configured")
		}
		code, err := generateTOTP(a.otpSecret)
		if err != nil {
			return fmt.Errorf("amazon: generate OTP: %w", err)
		}
		if err := pg.Fill(ctx, otpFieldSelector, code); err != nil {
			return fmt.Errorf("amazon: fill OTP: %w", err)
		}
		if err := pg.Click(ctx, otpSubmitBtn); err != nil {
			return fmt.Errorf("amazon: submit OTP: %w", err)
		}
		if err := ctxSleep(ctx, loginPollWait); err != nil {
			return err
		}
	}

	// Verify we're actually logged in now.
	html, err = pg.Content(ctx)
	if err != nil {
		return fmt.Errorf("amazon: read final page: %w", err)
	}
	if isLoginPage(html) {
		return fmt.Errorf("amazon: login failed — still on sign-in page (wrong credentials or CAPTCHA)")
	}

	return nil
}

// ensureSession returns the existing session or creates a new one (without cookies — login will populate them).
func (a *amazon) ensureSession(ctx context.Context) (mcp.BrowserSession, error) {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	if a.session != nil {
		return a.session, nil
	}
	if a.browserSvc == nil {
		return nil, fmt.Errorf("amazon: browser service not available")
	}
	sess, err := a.browserSvc.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("amazon: create browser session: %w", err)
	}
	// Inject seed cookies if available (helps avoid some bot detection).
	if len(a.browserCookies) > 0 {
		if err := sess.AddCookies(ctx, a.browserCookies); err != nil {
			_ = sess.Close()
			return nil, fmt.Errorf("amazon: inject cookies: %w", err)
		}
	}
	a.session = sess
	return sess, nil
}

// ctxSleep waits for d or until ctx is cancelled, whichever comes first.
func ctxSleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// --- TOTP (RFC 6238) ---

func generateTOTP(secret string) (string, error) {
	secret = strings.TrimSpace(strings.ToUpper(secret))
	secret = strings.ReplaceAll(secret, " ", "")
	secret = strings.TrimRight(secret, "=")
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("invalid base32 OTP secret: %w", err)
	}

	counter := uint64(math.Floor(float64(time.Now().Unix()) / 30))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1000000), nil
}
