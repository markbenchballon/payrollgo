package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int
	Username string
	Name     string
	Password string
	Gmail    string
	Role     string
}

type EmployeeInfo struct {
	EmployeeID    string
	FirstName     string
	LastName      string
	Position      string
	Department    string
	Email         string
	ContactNumber string
	DateHired     string
	BasicSalary   float64
	Role          string
	Gender        string
}

var db *sql.DB

var schedulerOnce sync.Once

func startPayrollReminderScheduler() {
	schedulerOnce.Do(func() {
		go func() {
			fmt.Println("📅 Payroll reminder scheduler started...")
			for {
				now := time.Now()

				// Kunin ang huling araw ng current month
				lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())

				// Check kung today ay last day ng month
				isLastDay := now.Day() == lastDay.Day() &&
					now.Month() == lastDay.Month() &&
					now.Year() == lastDay.Year()

				if isLastDay {
					fmt.Printf("📬 Today is the last day of %s — sending payroll reminders...\n",
						now.Month().String())
					sendPayrollReminders()
				}

				// Mag-check ulit bukas ng 8:00 AM
				tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 8, 0, 0, 0, now.Location())
				sleepDuration := time.Until(tomorrow)

				fmt.Printf("⏰ Next payroll reminder check: %s (in %.1f hours)\n",
					tomorrow.Format("Jan 02, 2006 8:00 AM"),
					sleepDuration.Hours())

				time.Sleep(sleepDuration)
			}
		}()
	})
}

func sendPayrollReminders() {
	now := time.Now()
	monthName := now.Month().String()
	year := now.Year()

	rows, err := db.Query(`
		SELECT e.employee_id, e.first_name, e.last_name, e.email
		FROM employees e
		WHERE e.email IS NOT NULL
		AND e.email != ''
		AND NOT EXISTS (
			SELECT 1 FROM payroll p
			WHERE p.employee_id = e.employee_id
			AND MONTH(p.period_from) = ?
			AND YEAR(p.period_from) = ?
		)
	`, int(now.Month()), year)

	if err != nil {
		fmt.Printf("❌ Error fetching employees for reminder: %v\n", err)
		return
	}
	defer rows.Close()

	type EmployeeReminder struct {
		EmployeeID string
		FirstName  string
		LastName   string
		Email      string
	}

	var employees []EmployeeReminder
	for rows.Next() {
		var emp EmployeeReminder
		if err := rows.Scan(&emp.EmployeeID, &emp.FirstName, &emp.LastName, &emp.Email); err != nil {
			continue
		}
		employees = append(employees, emp)
	}

	if len(employees) == 0 {
		fmt.Println("✅ All employees have already submitted payroll this month. No reminders needed.")
		return
	}

	fmt.Printf("📧 Sending payroll reminders to %d employee(s)...\n", len(employees))

	successCount := 0
	failCount := 0

	for _, emp := range employees {
		err := sendPayrollReminderEmail(emp.Email, emp.FirstName, emp.LastName, monthName, year)
		if err != nil {
			fmt.Printf("❌ Failed to send reminder to %s (%s): %v\n", emp.EmployeeID, emp.Email, err)
			failCount++
		} else {
			fmt.Printf("✅ Reminder sent to %s %s (%s)\n", emp.FirstName, emp.LastName, emp.Email)
			successCount++
		}

		time.Sleep(1 * time.Second)
	}

	fmt.Printf("📊 Payroll reminder summary: %d sent, %d failed\n", successCount, failCount)
}

func sendPayrollReminderEmail(to, firstName, lastName, monthName string, year int) error {
	from := os.Getenv("SMTP_EMAIL")
	password := os.Getenv("SMTP_PASSWORD")

	if from == "" || password == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	subject := fmt.Sprintf("⚠️ Payroll Reminder: Please submit your %s %d payroll", monthName, year)

	body := fmt.Sprintf(`Dear %s %s,

This is a friendly reminder that today is the last day of %s %d.

📋 PAYROLL SUBMISSION REMINDER
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Our records show that you have NOT yet submitted your payroll for %s %d.

Please log in to the Payroll System as soon as possible and submit your payroll before the month ends.

Steps to submit:
1. Log in at the Payroll System
2. Click "Create Payroll" tab
3. Fill in your Days Worked and Period Covered
4. Click "Save Payroll"

⚠️  Failure to submit on time may delay your salary processing.

If you have already submitted or believe this is an error, please ignore this email or contact HR.

Thank you,
Payroll System
HR Department`,
		firstName, lastName,
		monthName, year,
		monthName, year,
	)

	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n\r\n" +
			body,
	)

	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, msg)
}

func ensureAdmin() {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE username='admin'",
	).Scan(&count)
	if err != nil {
		panic(err)
	}

	password := "admin123"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 12)

	if count == 0 {
		_, err := db.Exec(
			"INSERT INTO users (username, name, password, gmail, role) VALUES (?, ?, ?, ?, ?)",
			"admin",
			"System Admin",
			hashedPassword,
			"admin@gmail.com",
			"admin",
		)
		if err != nil {
			panic(err)
		}
		fmt.Println("✅ Admin account created")
	} else {
		_, err := db.Exec(
			"UPDATE users SET password=?, role='admin' WHERE username='admin'",
			hashedPassword,
		)
		if err != nil {
			panic(err)
		}
		fmt.Println("🔁 Admin password reset to admin123")
	}
}

func main() {
	var err error

	db, err = sql.Open("mysql", "root@tcp(127.0.0.1:3306)/payrollgo")
	if err != nil {
		panic(err)
	}

	if err = db.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("Connected to MySQL payrollgo!")

	ensureAdmin()

	startPayrollReminderScheduler()
	fmt.Println("✅ Payroll reminder scheduler started!")

	r := gin.Default()

	store := cookie.NewStore([]byte("super-secret-key-123456"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
	})

	r.Use(sessions.Sessions("payroll_session", store))

	r.Static("/static", "./static")

	r.LoadHTMLGlob("templates/*")

	// Auth
	r.GET("/", showLoginPage)
	r.POST("/login", login)
	r.GET("/dashboard", authRequired(), dashboard)
	r.GET("/logout", logout)

	// Admin routes
	r.GET("/admin/employees", authRequired(), showAdminEmployeesPage)
	r.GET("/admin/payroll", authRequired(), showAdminPayrollPage) // ← DAGDAG MO ITO

	// Employee routes
	r.GET("/employee/dashboard", employeeDashboard)

	// Manager routes
	r.GET("/manager/dashboard", managerDashboard)

	// Supervisor routes
	r.GET("/supervisor/dashboard", supervisorDashboard)

	// Reports API Routes
	r.GET("/api/reports/payroll-summary", authRequired(), getPayrollSummaryReport)
	r.GET("/api/reports/employee-earnings", authRequired(), getEmployeeEarningsReport)
	r.GET("/api/reports/deductions", authRequired(), getDeductionsReport)
	r.GET("/api/reports/department-costs", authRequired(), getDepartmentCostsReport)
	r.GET("/api/reports/tax-summary", authRequired(), getTaxSummaryReport)
	r.GET("/api/reports/contribution-summary", authRequired(), getContributionSummaryReport)
	r.GET("/admin/reports", authRequired(), showAdminReportsPage)
	fmt.Println("✅ Reports route registered!")

	r.GET("/admin/settings", authRequired(), showAdminSettingsPage)
	r.POST("/admin/settings/job-grades", authRequired(), updateJobGrades)
	r.POST("/admin/settings/contributions", authRequired(), updateContributions)
	r.POST("/admin/settings/tax-brackets", authRequired(), updateTaxBrackets)
	r.POST("/admin/settings/company", authRequired(), updateCompanyInfo)

	r.GET("/api/payroll/details/:id", getPayrollDetails)

	r.GET("/users/add", authRequired(), showCreateUserPage)
	r.POST("/users/add", authRequired(), createUser)

	r.GET("/forgot-password", showForgotPasswordPage)
	r.POST("/forgot-password", sendOTP)
	r.GET("/reset-password", showResetPasswordPage)
	r.POST("/reset-password", resetPassword)

	// HR & User Management Routes
	RegisterHRRoutes(r)
	RegisterUserRoutes(r)
	RegisterEmployeeRoutes(r)

	r.Run(":8080")
}

func showAdminPayrollPage(c *gin.Context) {
	session := sessions.Default(c)

	role := session.Get("role")
	if role == nil || role.(string) != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "payroll.html", gin.H{
		"username": session.Get("username"),
		"date":     time.Now().Format("January 02, 2006"),
	})
}

// Get payroll details
func getPayrollDetails(c *gin.Context) {
	payrollID := c.Param("id")

	var p struct {
		ID                int
		EmployeeID        string
		FirstName         string
		LastName          string
		Position          string
		Department        string
		PeriodFrom        string
		PeriodTo          string
		DaysWorked        int
		TotalWorkingDays  int
		BasicSalary       float64
		GrossPay          float64
		SSSContribution   float64
		PhilHealthContrib float64
		PagIbigContrib    float64
		WithholdingTax    float64
		TotalDeductions   float64
		NetPay            float64
	}

	err := db.QueryRow(`
    SELECT 
        p.id, 
        p.employee_id, 
        e.first_name, 
        e.last_name, 
        e.position, 
        e.department,
        p.period_from, 
        p.period_to, 
        p.days_worked, 
        p.total_working_days,
        p.basic_salary, 
        p.gross_pay, 
        COALESCE(p.sss, 0), 
        COALESCE(p.philhealth, 0), 
        COALESCE(p.pagibig, 0), 
        COALESCE(p.tax, 0), 
        p.total_deductions, 
        p.net_pay
    FROM payroll p
    JOIN employees e ON p.employee_id = e.employee_id
    WHERE p.id = ?
`, payrollID).Scan(
		&p.ID, &p.EmployeeID, &p.FirstName, &p.LastName, &p.Position, &p.Department,
		&p.PeriodFrom, &p.PeriodTo, &p.DaysWorked, &p.TotalWorkingDays,
		&p.BasicSalary, &p.GrossPay, &p.SSSContribution, &p.PhilHealthContrib,
		&p.PagIbigContrib, &p.WithholdingTax, &p.TotalDeductions, &p.NetPay,
	)

	if err != nil {
		fmt.Println("❌ Payroll Details Error:", err)
		fmt.Println("❌ Payroll ID:", payrollID)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      p.ID,
		"employee_id":             p.EmployeeID,
		"first_name":              p.FirstName,
		"last_name":               p.LastName,
		"position":                p.Position,
		"department":              p.Department,
		"period_from":             p.PeriodFrom,
		"period_to":               p.PeriodTo,
		"days_worked":             p.DaysWorked,
		"total_working_days":      p.TotalWorkingDays,
		"basic_salary":            p.BasicSalary,
		"gross_pay":               p.GrossPay,
		"sss_contribution":        p.SSSContribution,
		"philhealth_contribution": p.PhilHealthContrib,
		"pagibig_contribution":    p.PagIbigContrib,
		"withholding_tax":         p.WithholdingTax,
		"total_deductions":        p.TotalDeductions,
		"net_pay":                 p.NetPay,
	})
}

// Show Admin Reports Page
func showAdminReportsPage(c *gin.Context) {
	session := sessions.Default(c)

	fmt.Println("🔍 Reports page accessed!")

	role := session.Get("role")
	fmt.Println("🔍 User role:", role)

	if role == nil || role.(string) != "admin" {
		fmt.Println("❌ Access denied - not admin")
		c.Redirect(http.StatusFound, "/")
		return
	}

	fmt.Println("✅ Loading reports.html")

	c.HTML(http.StatusOK, "reports.html", gin.H{
		"username": session.Get("username"),
		"date":     time.Now().Format("January 02, 2006"),
	})
}

/* ===========================
   REPORTS API FUNCTIONS
=========================== */

// Payroll Summary Report
func getPayrollSummaryReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")
	department := c.Query("department")

	query := `
		SELECT 
			DATE_FORMAT(p.period_from, '%b %d, %Y') as period_from,
			DATE_FORMAT(p.period_to, '%b %d, %Y') as period_to,
			COUNT(DISTINCT p.employee_id) as total_employees,
			SUM(p.gross_pay) as total_gross_pay,
			SUM(COALESCE(p.sss, 0)) as total_sss,
			SUM(COALESCE(p.philhealth, 0)) as total_philhealth,
			SUM(COALESCE(p.pagibig, 0)) as total_pagibig,
			SUM(COALESCE(p.tax, 0)) as total_tax,
			SUM(p.total_deductions) as total_deductions,
			SUM(p.net_pay) as total_net_pay
		FROM payroll p
		JOIN employees e ON p.employee_id = e.employee_id
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}
	if department != "" {
		query += " AND e.department = ?"
		args = append(args, department)
	}

	query += " GROUP BY p.period_from, p.period_to ORDER BY p.period_from DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	var grandTotalGross, grandTotalDeductions, grandTotalNet float64

	for rows.Next() {
		var periodFrom, periodTo string
		var totalEmployees int
		var totalGross, totalSSS, totalPhilHealth, totalPagibig, totalTax, totalDeductions, totalNet float64

		err := rows.Scan(&periodFrom, &periodTo, &totalEmployees, &totalGross, &totalSSS,
			&totalPhilHealth, &totalPagibig, &totalTax, &totalDeductions, &totalNet)
		if err != nil {
			continue
		}

		grandTotalGross += totalGross
		grandTotalDeductions += totalDeductions
		grandTotalNet += totalNet

		results = append(results, map[string]interface{}{
			"period":           periodFrom + " - " + periodTo,
			"employees":        totalEmployees,
			"gross_pay":        totalGross,
			"sss":              totalSSS,
			"philhealth":       totalPhilHealth,
			"pagibig":          totalPagibig,
			"tax":              totalTax,
			"total_deductions": totalDeductions,
			"net_pay":          totalNet,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": map[string]interface{}{
			"total_gross":      grandTotalGross,
			"total_deductions": grandTotalDeductions,
			"total_net":        grandTotalNet,
		},
		"records": results,
	})
}

// Employee Earnings Report
func getEmployeeEarningsReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")
	department := c.Query("department")

	query := `
		SELECT 
			e.employee_id,
			e.first_name,
			e.last_name,
			e.department,
			e.position,
			p.basic_salary,
			SUM(p.gross_pay) as total_gross_pay,
			COUNT(p.id) as payroll_count
		FROM employees e
		LEFT JOIN payroll p ON e.employee_id = p.employee_id
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}
	if department != "" {
		query += " AND e.department = ?"
		args = append(args, department)
	}

	query += " GROUP BY e.employee_id, e.first_name, e.last_name, e.department, e.position, p.basic_salary"
	query += " ORDER BY e.employee_id"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}

	for rows.Next() {
		var empID, firstName, lastName, dept, position string
		var basicSalary, totalGross float64
		var payrollCount int

		err := rows.Scan(&empID, &firstName, &lastName, &dept, &position,
			&basicSalary, &totalGross, &payrollCount)
		if err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"employee_id":   empID,
			"name":          firstName + " " + lastName,
			"department":    dept,
			"position":      position,
			"basic_salary":  basicSalary,
			"total_gross":   totalGross,
			"payroll_count": payrollCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"records": results})
}

// Deductions Report
func getDeductionsReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")
	department := c.Query("department")

	query := `
		SELECT 
			e.employee_id,
			e.first_name,
			e.last_name,
			e.department,
			SUM(COALESCE(p.sss, 0)) as total_sss,
			SUM(COALESCE(p.philhealth, 0)) as total_philhealth,
			SUM(COALESCE(p.pagibig, 0)) as total_pagibig,
			SUM(COALESCE(p.tax, 0)) as total_tax,
			SUM(p.total_deductions) as total_deductions
		FROM employees e
		LEFT JOIN payroll p ON e.employee_id = p.employee_id
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}
	if department != "" {
		query += " AND e.department = ?"
		args = append(args, department)
	}

	query += " GROUP BY e.employee_id, e.first_name, e.last_name, e.department"
	query += " ORDER BY e.employee_id"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	var totalSSS, totalPhilHealth, totalPagibig, totalTax float64

	for rows.Next() {
		var empID, firstName, lastName, dept string
		var sss, philhealth, pagibig, tax, totalDed float64

		err := rows.Scan(&empID, &firstName, &lastName, &dept, &sss, &philhealth, &pagibig, &tax, &totalDed)
		if err != nil {
			continue
		}

		totalSSS += sss
		totalPhilHealth += philhealth
		totalPagibig += pagibig
		totalTax += tax

		results = append(results, map[string]interface{}{
			"employee_id":      empID,
			"name":             firstName + " " + lastName,
			"department":       dept,
			"sss":              sss,
			"philhealth":       philhealth,
			"pagibig":          pagibig,
			"tax":              tax,
			"total_deductions": totalDed,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": map[string]interface{}{
			"total_sss":        totalSSS,
			"total_philhealth": totalPhilHealth,
			"total_pagibig":    totalPagibig,
			"total_tax":        totalTax,
		},
		"records": results,
	})
}

// Department Costs Report
func getDepartmentCostsReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")

	query := `
		SELECT 
			e.department,
			COUNT(DISTINCT e.employee_id) as total_employees,
			SUM(p.gross_pay) as total_gross_pay,
			SUM(p.total_deductions) as total_deductions,
			SUM(p.net_pay) as total_net_pay
		FROM employees e
		LEFT JOIN payroll p ON e.employee_id = p.employee_id
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}

	query += " GROUP BY e.department ORDER BY total_gross_pay DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	var grandTotal float64

	for rows.Next() {
		var dept string
		var empCount int
		var totalGross, totalDed, totalNet float64

		err := rows.Scan(&dept, &empCount, &totalGross, &totalDed, &totalNet)
		if err != nil {
			continue
		}

		grandTotal += totalGross

		results = append(results, map[string]interface{}{
			"department":       dept,
			"employees":        empCount,
			"total_gross_pay":  totalGross,
			"total_deductions": totalDed,
			"total_net_pay":    totalNet,
			"percentage":       0.0, // Will calculate after
		})
	}

	// Calculate percentages
	for i := range results {
		if grandTotal > 0 {
			results[i]["percentage"] = (results[i]["total_gross_pay"].(float64) / grandTotal) * 100
		}
	}

	c.JSON(http.StatusOK, gin.H{"records": results})
}

// Tax Summary Report
func getTaxSummaryReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")

	query := `
		SELECT 
			COUNT(DISTINCT p.employee_id) as total_employees,
			SUM(COALESCE(p.tax, 0)) as total_tax,
			AVG(COALESCE(p.tax, 0)) as avg_tax
		FROM payroll p
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}

	var totalEmployees int
	var totalTax, avgTax float64

	err := db.QueryRow(query, args...).Scan(&totalEmployees, &totalTax, &avgTax)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": map[string]interface{}{
			"total_employees": totalEmployees,
			"total_tax":       totalTax,
			"average_tax":     avgTax,
		},
	})
}

// Contribution Summary Report
func getContributionSummaryReport(c *gin.Context) {
	fromDate := c.Query("from")
	toDate := c.Query("to")

	query := `
		SELECT 
			SUM(COALESCE(p.sss, 0)) as employee_sss,
			SUM(COALESCE(p.philhealth, 0)) as employee_philhealth,
			SUM(COALESCE(p.pagibig, 0)) as employee_pagibig
		FROM payroll p
		WHERE 1=1
	`

	args := []interface{}{}

	if fromDate != "" {
		query += " AND p.period_from >= ?"
		args = append(args, fromDate)
	}
	if toDate != "" {
		query += " AND p.period_to <= ?"
		args = append(args, toDate)
	}

	var empSSS, empPhilHealth, empPagibig float64

	err := db.QueryRow(query, args...).Scan(&empSSS, &empPhilHealth, &empPagibig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Employer shares (based on Philippine labor law)
	empSSS_Employer := empSSS * 1.5         // Employer pays 1.5x employee share
	empPhilHealth_Employer := empPhilHealth // Equal share
	empPagibig_Employer := empPagibig       // Equal share

	results := []map[string]interface{}{
		{
			"type":             "SSS",
			"employee_share":   empSSS,
			"employer_share":   empSSS_Employer,
			"total_remittance": empSSS + empSSS_Employer,
		},
		{
			"type":             "PhilHealth",
			"employee_share":   empPhilHealth,
			"employer_share":   empPhilHealth_Employer,
			"total_remittance": empPhilHealth + empPhilHealth_Employer,
		},
		{
			"type":             "Pag-IBIG",
			"employee_share":   empPagibig,
			"employer_share":   empPagibig_Employer,
			"total_remittance": empPagibig + empPagibig_Employer,
		},
	}

	c.JSON(http.StatusOK, gin.H{"records": results})
}

func showAdminEmployeesPage(c *gin.Context) {
	employees := getAllEmployees()

	c.HTML(http.StatusOK, "employees.html", gin.H{
		"employees": employees,
	})
}

func showLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func login(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := strings.TrimSpace(c.PostForm("password"))

	// ── Username validation ──────────────────────────────────────
	if username == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Username is required.",
		})
		return
	}
	if len(username) > 30 {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Username must not exceed 30 characters.",
		})
		return
	}
	if strings.ContainsAny(username, " \t") {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Username must not contain spaces.",
		})
		return
	}
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validUsername.MatchString(username) {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Username must not contain special characters.",
		})
		return
	}

	if password == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Password is required.",
		})
		return
	}
	if len(password) > 30 {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Password must not exceed 30 characters.",
		})
		return
	}
	if strings.ContainsAny(password, " \t") {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Password must not contain spaces.",
		})
		return
	}
	validPassword := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !validPassword.MatchString(password) {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"error": "⚠️ Password must not contain special characters.",
		})
		return
	}

	row := db.QueryRow(
		"SELECT id, username, password, gmail, role FROM users WHERE username=?",
		username,
	)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.Gmail, &u.Role)
	if err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"error": "Invalid username or password",
		})
		return
	}

	role := strings.ToLower(strings.TrimSpace(u.Role))

	session := sessions.Default(c)
	session.Set("userID", u.ID)
	session.Set("username", u.Username)
	session.Set("role", role)

	// ✅ LOAD EMPLOYEE DATA IF ROLE IS EMPLOYEE/SUPERVISOR/MANAGER
	if role == "employee" || role == "supervisor" || role == "manager" {
		var emp EmployeeInfo
		var empStatus string

		empErr := db.QueryRow(`
			SELECT employee_id, first_name, last_name, position, department, 
			       email, contact, date_hired, basic_salary, role, gender, status
			FROM employees 
			WHERE email = ?
		`, u.Gmail).Scan(
			&emp.EmployeeID,
			&emp.FirstName,
			&emp.LastName,
			&emp.Position,
			&emp.Department,
			&emp.Email,
			&emp.ContactNumber,
			&emp.DateHired,
			&emp.BasicSalary,
			&emp.Role,
			&emp.Gender,
			&empStatus,
		)

		if empErr == nil {
			if strings.ToLower(empStatus) == "inactive" {
				session.Clear()
				session.Save()
				c.HTML(http.StatusForbidden, "login.html", gin.H{
					"error": "⚠️ Your account has been deactivated. Please contact HR for assistance.",
				})
				return
			}

			empRole := strings.ToLower(strings.TrimSpace(emp.Role))

			session.Set("employee_id", emp.EmployeeID)
			session.Set("employee_first_name", emp.FirstName)
			session.Set("employee_last_name", emp.LastName)
			session.Set("employee_position", emp.Position)
			session.Set("employee_department", emp.Department)
			session.Set("employee_email", emp.Email)
			session.Set("employee_contact", emp.ContactNumber)
			session.Set("employee_date_hired", emp.DateHired)
			session.Set("employee_basic_salary", emp.BasicSalary)
			session.Set("employee_role", empRole)
			session.Set("employee_gender", emp.Gender)
			session.Set("employee_status", empStatus)
		} else {
			fmt.Printf("❌ EMPLOYEE QUERY ERROR: %v\n", empErr)
		}
	}

	if err := session.Save(); err != nil {
		fmt.Println("❌ SESSION SAVE ERROR:", err)
		c.String(http.StatusInternalServerError, "Session error")
		return
	}

	fmt.Println("✅ LOGIN SUCCESS:", u.Username, "ROLE:", role)

	switch role {
	case "admin":
		c.Redirect(http.StatusFound, "/dashboard")
	case "hr":
		c.Redirect(http.StatusFound, "/hr/dashboard")
	case "manager":
		c.Redirect(http.StatusFound, "/manager/dashboard")
	case "supervisor":
		c.Redirect(http.StatusFound, "/supervisor/dashboard")
	case "employee":
		c.Redirect(http.StatusFound, "/employee/dashboard")
	default:
		fmt.Println("⚠️ UNKNOWN ROLE:", role)
		c.Redirect(http.StatusFound, "/")
	}
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusFound, "/")
}

func authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		if session.Get("userID") == nil {
			fmt.Println("❌ AUTH FAILED – NO SESSION")
			c.Redirect(http.StatusFound, "/")
			c.Abort()
			return
		}

		c.Next()
	}
}

/* ===========================
   DASHBOARD FUNCTIONS
=========================== */

func dashboard(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	// Redirect based on role
	switch role {
	case "employee":
		c.Redirect(http.StatusFound, "/employee/dashboard")
		return
	case "hr":
		c.Redirect(http.StatusFound, "/hr/dashboard")
		return
	case "manager":
		c.Redirect(http.StatusFound, "/manager/dashboard")
		return
	case "supervisor":
		c.Redirect(http.StatusFound, "/employee/dashboard")
		return

	}

	// Admin dashboard
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"username":       session.Get("username"),
		"role":           role,
		"totalUsers":     countUsers(),
		"totalEmployees": countEmployees(),
		"date":           time.Now().Format("January 02, 2006"),
	})
}

func employeeDashboard(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	roleStr := role.(string)

	// Allow both employee and supervisor
	if roleStr != "employee" && roleStr != "supervisor" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	isSupervisor := false
	if roleStr == "supervisor" {
		isSupervisor = true
	}

	c.HTML(http.StatusOK, "employee_dashboard.html", gin.H{
		"username":     session.Get("username"),
		"date":         time.Now().Format("January 02, 2006"),
		"isSupervisor": isSupervisor,
	})
}

func managerDashboard(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil || role.(string) != "manager" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "manager_dashboard.html", gin.H{
		"username":          session.Get("username"),
		"date":              time.Now().Format("January 02, 2006"),
		"totalEmployees":    countEmployees(),
		"pendingApprovals":  countPendingApprovals(),
		"departmentReports": getDepartmentReports(),
	})
}

// ✅ NEW CODE - CORRECTED SUPERVISOR DASHBOARD
func supervisorDashboard(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil || role.(string) != "supervisor" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// ✅ EXPLICITLY SET isSupervisor = true
	c.HTML(http.StatusOK, "employee_dashboard.html", gin.H{
		"username":       session.Get("username"),
		"date":           time.Now().Format("January 02, 2006"),
		"isSupervisor":   true, // ← IMPORTANTE ITO!
		"totalEmployees": countEmployees(),
		"teamMembers":    getTeamMembers(),
		"todayPresent":   countTodayPresent(),
	})
}

/* ===========================
   HELPER FUNCTIONS
=========================== */

func countUsers() int {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)
	return total
}

func countEmployees() int {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM employees").Scan(&total)
	return total
}

func countPendingApprovals() int {
	var total int
	// You can customize this based on your approval system
	db.QueryRow("SELECT COUNT(*) FROM payroll WHERE status = 'pending'").Scan(&total)
	return total
}

func countTodayPresent() int {
	var total int
	// Placeholder - customize based on your attendance system
	return total
}

func getDepartmentReports() []map[string]interface{} {
	// Placeholder - return department statistics
	return []map[string]interface{}{}
}

func getTeamMembers() []map[string]interface{} {
	// Placeholder - return team member list
	return []map[string]interface{}{}
}

/* ===========================
   OTP / PASSWORD RESET
=========================== */

/* ===========================
   SETTINGS PAGE & FUNCTIONS
=========================== */

func showAdminSettingsPage(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil || role.(string) != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "settings.html", gin.H{
		"username": session.Get("username"),
		"date":     time.Now().Format("January 02, 2006"),
	})
}

/* ===========================
   ADD USER FUNCTIONS
=========================== */

func showCreateUserPage(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil || role.(string) != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "create_user.html", gin.H{
		"username": session.Get("username"),
		"date":     time.Now().Format("January 02, 2006"),
	})
}

func createUser(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("role")

	if role == nil || role.(string) != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	username := strings.TrimSpace(c.PostForm("username"))
	name := strings.TrimSpace(c.PostForm("name"))
	gmail := strings.TrimSpace(c.PostForm("gmail"))
	password := strings.TrimSpace(c.PostForm("password"))
	userRole := strings.TrimSpace(c.PostForm("role"))

	// Validation
	if username == "" || name == "" || gmail == "" || password == "" || userRole == "" {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "All fields are required",
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2006"),
		})
		return
	}

	// Check if username already exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username=?", username).Scan(&count)
	if err != nil {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "Database error",
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2006"),
		})
		return
	}

	if count > 0 {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "Username already exists",
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2006"),
		})
		return
	}

	// Check if password is at least 6 characters
	if len(password) < 6 {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "Password must be at least 6 characters",
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2026"),
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "Error creating password",
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2006"),
		})
		return
	}

	// Insert user
	_, err = db.Exec(
		"INSERT INTO users (username, name, password, gmail, role) VALUES (?, ?, ?, ?, ?)",
		username, name, hashedPassword, gmail, userRole,
	)

	if err != nil {
		c.HTML(http.StatusOK, "create_user.html", gin.H{
			"error":    "Error creating user: " + err.Error(),
			"username": session.Get("username"),
			"date":     time.Now().Format("January 02, 2006"),
		})
		return
	}

	fmt.Printf("✅ User created: %s (%s) - Role: %s\n", username, name, userRole)
	c.Redirect(http.StatusFound, "/users?success=User+created+successfully")
}

func updateJobGrades(c *gin.Context) {
	// Handle job grade salary updates
	grade := c.PostForm("grade")
	title := c.PostForm("title")
	salary := c.PostForm("salary")

	_, err := db.Exec(`
		INSERT INTO job_grades (grade, title, salary) 
		VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE title=?, salary=?
	`, grade, title, salary, title, salary)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateContributions(c *gin.Context) {
	// Handle SSS, PhilHealth, Pag-IBIG rate updates
	sssRate := c.PostForm("sss_rate")
	philhealthRate := c.PostForm("philhealth_rate")
	pagibigRate := c.PostForm("pagibig_rate")

	_, err := db.Exec(`
		UPDATE system_settings 
		SET sss_rate=?, philhealth_rate=?, pagibig_rate=?
		WHERE id=1
	`, sssRate, philhealthRate, pagibigRate)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func updateTaxBrackets(c *gin.Context) {
	// Handle withholding tax bracket updates
	c.JSON(http.StatusOK, gin.H{"message": "Tax brackets updated"})
}

func updateCompanyInfo(c *gin.Context) {
	companyName := c.PostForm("company_name")
	address := c.PostForm("address")
	tin := c.PostForm("tin")

	_, err := db.Exec(`
		UPDATE system_settings 
		SET company_name=?, company_address=?, company_tin=?
		WHERE id=1
	`, companyName, address, tin)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
