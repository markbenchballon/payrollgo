package main

import (
	crand "crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

/* ===========================
   FORGOT PASSWORD FUNCTIONS
=========================== */

// Generate a 6-digit OTP
func generateOTP() string {
	n, err := crand.Int(crand.Reader, big.NewInt(1000000))
	if err != nil {
		// Fallback to timestamp-based OTP if crypto/rand fails
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// Send email with OTP
func sendEmail(to, otp string) error {
	// Use proper environment variable names
	from := "glensssg@gmail.com"
	password := "ueaejehhkllpvlgv"

	if from == "" || password == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	subject := "Password Reset OTP - Payroll System"

	body := fmt.Sprintf(`Dear User,

You have requested to reset your password for the Payroll System.

Your One-Time Password (OTP) is: %s

This code will expire in 5 minutes.

If you did not request this password reset, please ignore this email or contact your system administrator.

Best regards,
Payroll System Team`, otp)

	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n\r\n" +
			body,
	)

	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, msg)
}

// Mask email for privacy (show only first 2 chars)
func maskEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "your email"
	}
	local := parts[0]
	if len(local) <= 2 {
		return local + "@" + parts[1]
	}
	return local[:2] + strings.Repeat("*", len(local)-2) + "@" + parts[1]
}

// Show forgot password page
func showForgotPasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot_password.html", nil)
}

// Handle OTP sending
func sendOTP(c *gin.Context) {
	identifier := strings.TrimSpace(c.PostForm("identifier"))

	if identifier == "" {
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"error": "Please enter your username or email address.",
		})
		return
	}

	var id int
	var email string

	// Find user by username OR email
	err := db.QueryRow(
		"SELECT id, gmail FROM users WHERE username=? OR gmail=?",
		identifier, identifier,
	).Scan(&id, &email)

	if err != nil {
		fmt.Printf("❌ User not found: %s\n", identifier)
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"error": "No account found with that username or email.",
		})
		return
	}

	if email == "" {
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"error": "This account does not have an email address registered. Please contact your administrator.",
		})
		return
	}

	// Generate OTP and set expiry (5 minutes from now)
	otp := generateOTP()
	expiry := time.Now().Add(5 * time.Minute)

	// Save OTP to database
	_, dbErr := db.Exec(
		"UPDATE users SET reset_otp=?, otp_expiry=? WHERE id=?",
		otp, expiry, id,
	)
	if dbErr != nil {
		fmt.Printf("❌ Database error: %v\n", dbErr)
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"error": "Database error. Please try again.",
		})
		return
	}

	// Send OTP via email
	if err := sendEmail(email, otp); err != nil {
		fmt.Printf("❌ Email send error: %v\n", err)
		c.HTML(http.StatusOK, "forgot_password.html", gin.H{
			"error": "Failed to send OTP email. Please check your SMTP settings or try again later.",
		})
		return
	}

	// Store email in session for pre-filling reset page
	session := sessions.Default(c)
	session.Set("reset_email", email)
	session.Save()

	fmt.Printf("✅ OTP sent to %s\n", maskEmail(email))

	c.HTML(http.StatusOK, "forgot_password.html", gin.H{
		"success": "OTP sent to " + maskEmail(email) + ". Please check your inbox.",
	})
}

// Show reset password page
func showResetPasswordPage(c *gin.Context) {
	session := sessions.Default(c)
	prefillEmail := ""
	if v := session.Get("reset_email"); v != nil {
		prefillEmail = v.(string)
	}
	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"prefill_email": prefillEmail,
	})
}

// Handle password reset
func resetPassword(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("gmail"))
	otp := strings.TrimSpace(c.PostForm("otp"))
	newPassword := strings.TrimSpace(c.PostForm("password"))
	confirmPassword := strings.TrimSpace(c.PostForm("confirm_password"))

	// Validation
	if email == "" || otp == "" || newPassword == "" {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "All fields are required.",
			"prefill_email": email,
		})
		return
	}

	if newPassword != confirmPassword {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Passwords do not match.",
			"prefill_email": email,
		})
		return
	}

	if len(newPassword) < 6 {
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Password must be at least 6 characters.",
			"prefill_email": email,
		})
		return
	}

	// ✅ FIX: Scan OTP and expiry as strings, then parse
	var dbOTP sql.NullString
	var expiryStr sql.NullString // ← Changed from sql.NullTime
	var id int

	err := db.QueryRow(
		"SELECT id, reset_otp, otp_expiry FROM users WHERE gmail=?",
		email,
	).Scan(&id, &dbOTP, &expiryStr) // ← Scan to string

	if err != nil {
		fmt.Printf("❌ Email not found: %s (Error: %v)\n", email, err)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Email address not found.",
			"prefill_email": email,
		})
		return
	}

	// Check if OTP exists (not NULL)
	if !dbOTP.Valid || !expiryStr.Valid {
		fmt.Printf("❌ No OTP set for %s\n", email)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "No OTP found. Please request a new one first.",
			"prefill_email": email,
		})
		return
	}

	// Verify OTP
	if dbOTP.String != otp {
		fmt.Printf("❌ Invalid OTP for %s\n", email)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Incorrect OTP. Please check your email and try again.",
			"prefill_email": email,
		})
		return
	}

	// ✅ Parse expiry string to time.Time
	expiry, err := time.Parse("2006-01-02 15:04:05", expiryStr.String)
	if err != nil {
		fmt.Printf("❌ Failed to parse expiry time: %v\n", err)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Invalid OTP expiry format. Please request a new OTP.",
			"prefill_email": email,
		})
		return
	}

	// Check if OTP expired
	if time.Now().After(expiry) {
		fmt.Printf("❌ Expired OTP for %s\n", email)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "OTP has expired. Please request a new one.",
			"prefill_email": email,
		})
		return
	}

	// Hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		fmt.Printf("❌ Bcrypt error: %v\n", err)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Internal error. Please try again.",
			"prefill_email": email,
		})
		return
	}

	// Update password and clear OTP
	_, dbErr := db.Exec(
		"UPDATE users SET password=?, reset_otp=NULL, otp_expiry=NULL WHERE id=?",
		hashed, id,
	)
	if dbErr != nil {
		fmt.Printf("❌ Database update error: %v\n", dbErr)
		c.HTML(http.StatusOK, "reset_password.html", gin.H{
			"error":         "Database error. Please try again.",
			"prefill_email": email,
		})
		return
	}

	// Clear session
	session := sessions.Default(c)
	session.Delete("reset_email")
	session.Save()

	fmt.Printf("✅ Password reset successful for %s\n", email)

	c.HTML(http.StatusOK, "reset_password.html", gin.H{
		"success": "✅ Password successfully changed! You can now log in with your new password.",
	})
}
