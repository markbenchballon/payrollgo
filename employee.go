package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type Leave struct {
	ID         int    `json:"id"`
	EmployeeID string `json:"employee_id"`
	LeaveType  string `json:"leave_type"`
	LeaveFrom  string `json:"leave_from"`
	LeaveTo    string `json:"leave_to"`
	LeaveDays  int    `json:"leave_days"`
	Reason     string `json:"reason"`

	ApproverRole      string `json:"approver_role"`
	ApproverDashboard string `json:"approver_dashboard"`

	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// EmployeeInfoAPI represents the employee information for the dashboard
type EmployeeInfoAPI struct {
	ID            int       `json:"id"`
	EmployeeID    string    `json:"employee_id"`
	Department    string    `json:"department"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Email         string    `json:"email"`
	ContactNumber string    `json:"contact"`
	Position      string    `json:"position"`
	DateHired     string    `json:"date_hired"`
	BasicSalary   float64   `json:"basic_salary"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Payroll represents payroll data - UPDATED WITH NEW FIELDS
type Payroll struct {
	ID               int     `json:"id"`
	EmployeeID       string  `json:"employee_id"`
	BasicSalary      float64 `json:"basic_salary"`
	TotalWorkingDays int     `json:"total_working_days"`
	DaysWorked       int     `json:"days_worked"`
	ProratedSalary   float64 `json:"prorated_salary"`

	// ✅ ADD THESE TWO NEW FIELDS
	RegularHolidayDate string  `json:"regular_holiday_date"`
	SpecialHolidayDate string  `json:"special_holiday_date"`
	RegularHolidayDays int     `json:"regular_holiday_days"`
	SpecialHolidayDays int     `json:"special_holiday_days"`
	RegularHolidayPay  float64 `json:"regular_holiday_pay"`
	SpecialHolidayPay  float64 `json:"special_holiday_pay"`

	// Leave Information
	SickLeave      float64 `json:"sick_leave"`
	VacationLeave  float64 `json:"vacation_leave"`
	MaternityLeave float64 `json:"maternity_leave"`
	PaternityLeave float64 `json:"paternity_leave"`
	OtherLeave     float64 `json:"other_leave"`

	// Allowances
	Transportation float64 `json:"transportation"`
	MealAllowance  float64 `json:"meal_allowance"`
	Communication  float64 `json:"communication"`
	OtherAllowance float64 `json:"other_allowance"`

	// Deductions
	SSS             float64 `json:"sss"`
	PhilHealth      float64 `json:"philhealth"`
	PagIBIG         float64 `json:"pagibig"`
	Tax             float64 `json:"tax"`
	CashAdvance     float64 `json:"cash_advance"`
	OtherDeductions float64 `json:"other_deductions"`

	// Computed
	GrossPay        float64 `json:"gross_pay"`
	TotalDeductions float64 `json:"total_deductions"`
	NetPay          float64 `json:"net_pay"`

	// Period
	PeriodFrom string    `json:"period_from"`
	PeriodTo   string    `json:"period_to"`
	CreatedAt  time.Time `json:"created_at"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ContributionResponse for auto-computed contributions
type ContributionResponse struct {
	BasicSalary float64 `json:"basic_salary"`
	SSS         float64 `json:"sss"`
	PhilHealth  float64 `json:"philhealth"`
	PagIBIG     float64 `json:"pagibig"`
	Tax         float64 `json:"tax"`
}

// PayrollMessage struct
type PayrollMessage struct {
	ID         int       `json:"id"`
	PayrollID  int       `json:"payroll_id"`
	EmployeeID string    `json:"employee_id"`
	SenderRole string    `json:"sender_role"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
	SenderName string    `json:"sender_name"`
}

// SendPayrollMessage - Employee sends concern to supervisor
func SendPayrollMessage(c *gin.Context) {
	var req struct {
		PayrollID int    `json:"payroll_id"`
		Message   string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	session := sessions.Default(c)
	employeeID := session.Get("employee_id")

	if employeeID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
		return
	}

	// Verify payroll belongs to employee
	var payrollEmployeeID string
	err := db.QueryRow("SELECT employee_id FROM payroll WHERE id = ?", req.PayrollID).Scan(&payrollEmployeeID)
	if err != nil || payrollEmployeeID != employeeID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "You can only message about your own payroll",
		})
		return
	}

	// Insert message
	_, err = db.Exec(`
		INSERT INTO payroll_messages (payroll_id, employee_id, sender_role, message, created_at)
		VALUES (?, ?, 'Employee', ?, NOW())
	`, req.PayrollID, employeeID, req.Message)

	if err != nil {
		log.Printf("❌ Error sending message: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send message",
		})
		return
	}

	log.Printf("✅ Employee %s sent message about payroll #%d", employeeID, req.PayrollID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message sent to supervisor",
	})
}

// GetPayrollMessages - Get messages for a specific payroll
func GetPayrollMessages(c *gin.Context) {
	payrollID := c.Param("payroll_id")

	session := sessions.Default(c)
	employeeID := session.Get("employee_id")
	role := session.Get("employee_role")

	// ✅ Check if user is HR
	username := session.Get("username")
	userRole := session.Get("role")

	// ✅ ALLOW HR TO VIEW ALL MESSAGES
	if username != nil && userRole != nil && userRole.(string) == "hr" {
		log.Printf("✅ HR user accessing payroll messages: %v", username)
	} else if employeeID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
		return
	} else if role != "Supervisor" && role != "Manager" {
		var payrollEmployeeID string
		err := db.QueryRow("SELECT employee_id FROM payroll WHERE id = ?", payrollID).Scan(&payrollEmployeeID)
		if err != nil || payrollEmployeeID != employeeID.(string) {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Access denied",
			})
			return
		}
	}

	// ✅ FIXED QUERY - Show HR name properly
	query := `
		SELECT 
			pm.id, 
			pm.payroll_id, 
			pm.employee_id, 
			pm.sender_role, 
			pm.message,
			DATE_FORMAT(pm.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
			CASE
				WHEN pm.sender_role = 'HR' THEN (
					SELECT CONCAT(name, ' (HR)') 
					FROM users 
					WHERE username = pm.employee_id 
					LIMIT 1
				)
				WHEN pm.sender_role = 'Manager' THEN (
					SELECT CONCAT(first_name, ' ', last_name, ' (Manager)') 
					FROM employees 
					WHERE employee_id = pm.employee_id 
					LIMIT 1
				)
				ELSE CONCAT(e.first_name, ' ', e.last_name)
			END as sender_name
		FROM payroll_messages pm
		LEFT JOIN employees e ON pm.employee_id = e.employee_id
		WHERE pm.payroll_id = ?
		ORDER BY pm.created_at ASC
	`

	rows, err := db.Query(query, payrollID)
	if err != nil {
		log.Printf("❌ Error fetching messages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to load messages",
		})
		return
	}
	defer rows.Close()

	var messages []PayrollMessage
	for rows.Next() {
		var msg PayrollMessage
		var createdAtStr string

		err := rows.Scan(&msg.ID, &msg.PayrollID, &msg.EmployeeID, &msg.SenderRole,
			&msg.Message, &createdAtStr, &msg.SenderName)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		layout := "2006-01-02 15:04:05"
		msg.CreatedAt, _ = time.Parse(layout, createdAtStr)

		messages = append(messages, msg)
	}

	log.Printf("✅ Returning %d messages for payroll #%s", len(messages), payrollID)

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"messages": messages,
	})
}

// ManagerReplyMessage - MANAGER replies to employee/supervisor concerns
func ManagerReplyMessage(c *gin.Context) {
	var req struct {
		PayrollID int    `json:"payroll_id"`
		Message   string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	session := sessions.Default(c)
	managerID := session.Get("employee_id")
	role := session.Get("employee_role")

	// ✅ ONLY MANAGERS CAN REPLY
	if managerID == nil || role != "Manager" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only managers can reply to concerns",
		})
		return
	}

	// Insert reply
	_, err := db.Exec(`
		INSERT INTO payroll_messages (payroll_id, employee_id, sender_role, message, created_at)
		VALUES (?, ?, 'Manager', ?, NOW())
	`, req.PayrollID, managerID, req.Message)

	if err != nil {
		log.Printf("❌ Error sending reply: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send reply",
		})
		return
	}

	log.Printf("✅ Manager %s replied to payroll #%d", managerID, req.PayrollID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reply sent successfully",
	})
}

// GetManagerPayrollConcerns - Get all payroll messages FOR MANAGER
func GetManagerPayrollConcerns(c *gin.Context) {
	session := sessions.Default(c)
	role := session.Get("employee_role")

	log.Printf("🔍 GetManagerPayrollConcerns called by role: %v", role)

	query := `
		SELECT DISTINCT
			p.id as payroll_id,
			p.employee_id,
			CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			p.period_from,
			p.period_to,
			p.net_pay,
			COUNT(pm.id) as message_count,
			MAX(pm.created_at) as last_message_time
		FROM payroll_messages pm
		JOIN payroll p ON pm.payroll_id = p.id
		JOIN employees e ON p.employee_id = e.employee_id
		GROUP BY p.id, p.employee_id, employee_name, p.period_from, p.period_to, p.net_pay
		ORDER BY last_message_time DESC
	`

	log.Printf("📊 Executing query to fetch payroll concerns...")

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("❌ Error fetching concerns: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to load concerns",
		})
		return
	}
	defer rows.Close()

	type PayrollConcern struct {
		PayrollID       int       `json:"payroll_id"`
		EmployeeID      string    `json:"employee_id"`
		EmployeeName    string    `json:"employee_name"`
		PeriodFrom      string    `json:"period_from"`
		PeriodTo        string    `json:"period_to"`
		NetPay          float64   `json:"net_pay"`
		MessageCount    int       `json:"message_count"`
		LastMessageTime time.Time `json:"last_message_time"`
	}

	var concerns []PayrollConcern
	for rows.Next() {
		var concern PayrollConcern
		err := rows.Scan(&concern.PayrollID, &concern.EmployeeID, &concern.EmployeeName,
			&concern.PeriodFrom, &concern.PeriodTo, &concern.NetPay,
			&concern.MessageCount, &concern.LastMessageTime)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}
		log.Printf("✅ Found concern: Payroll #%d - %s (%d messages)",
			concern.PayrollID, concern.EmployeeName, concern.MessageCount)
		concerns = append(concerns, concern)
	}

	log.Printf("✅ Total concerns found: %d", len(concerns))

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"concerns": concerns,
	})
}

// ============================================
// PHILIPPINE TAX & CONTRIBUTION COMPUTATIONS
// ============================================

// Compute SSS Contribution (2024 Table)
func computeSSS(monthlySalary float64) float64 {
	// SSS Contribution Table 2024 (Employee Share)
	switch {
	case monthlySalary <= 4250:
		return 180.00
	case monthlySalary <= 4750:
		return 202.50
	case monthlySalary <= 5250:
		return 225.00
	case monthlySalary <= 5750:
		return 247.50
	case monthlySalary <= 6250:
		return 270.00
	case monthlySalary <= 6750:
		return 292.50
	case monthlySalary <= 7250:
		return 315.00
	case monthlySalary <= 7750:
		return 337.50
	case monthlySalary <= 8250:
		return 360.00
	case monthlySalary <= 8750:
		return 382.50
	case monthlySalary <= 9250:
		return 405.00
	case monthlySalary <= 9750:
		return 427.50
	case monthlySalary <= 10250:
		return 450.00
	case monthlySalary <= 10750:
		return 472.50
	case monthlySalary <= 11250:
		return 495.00
	case monthlySalary <= 11750:
		return 517.50
	case monthlySalary <= 12250:
		return 540.00
	case monthlySalary <= 12750:
		return 562.50
	case monthlySalary <= 13250:
		return 585.00
	case monthlySalary <= 13750:
		return 607.50
	case monthlySalary <= 14250:
		return 630.00
	case monthlySalary <= 14750:
		return 652.50
	case monthlySalary <= 15250:
		return 675.00
	case monthlySalary <= 15750:
		return 697.50
	case monthlySalary <= 16250:
		return 720.00
	case monthlySalary <= 16750:
		return 742.50
	case monthlySalary <= 17250:
		return 765.00
	case monthlySalary <= 17750:
		return 787.50
	case monthlySalary <= 18250:
		return 810.00
	case monthlySalary <= 18750:
		return 832.50
	case monthlySalary <= 19250:
		return 855.00
	case monthlySalary <= 19750:
		return 877.50
	default:
		return 900.00 // Maximum SSS contribution
	}
}

// Compute PhilHealth Contribution (2024)
func computePhilHealth(monthlySalary float64) float64 {
	// PhilHealth Premium Rate: 5% of basic salary
	// Employee share: 2.5% (half of 5%)
	// Minimum: ₱10,000, Maximum: ₱100,000

	baseSalary := monthlySalary
	if baseSalary < 10000 {
		baseSalary = 10000
	}
	if baseSalary > 100000 {
		baseSalary = 100000
	}

	return math.Round((baseSalary*0.05/2)*100) / 100 // Employee share = 2.5%
}

// Compute Pag-IBIG Contribution (2024)
func computePagIBIG(monthlySalary float64) float64 {
	// Pag-IBIG: 2% of monthly salary
	// Maximum salary ceiling: ₱5,000

	if monthlySalary <= 1500 {
		return monthlySalary * 0.01 // 1% for ₱1,500 and below
	}

	baseSalary := monthlySalary
	if baseSalary > 5000 {
		baseSalary = 5000 // Maximum
	}

	return math.Round(baseSalary*0.02*100) / 100
}

// Compute Withholding Tax (2024 TRAIN Law - Pro-rated)
func computeWithholdingTax(monthlySalary float64, daysWorked int, totalWorkingDays int) float64 {
	// ✅ PRO-RATE THE SALARY FIRST
	proratedSalary := (monthlySalary / float64(totalWorkingDays)) * float64(daysWorked)

	// Annual tax computation based on pro-rated salary
	annualSalary := proratedSalary * 12

	// Deduct mandatory contributions (also pro-rated)
	monthlySSS := computeSSS(monthlySalary)
	monthlyPhilHealth := computePhilHealth(monthlySalary)
	monthlyPagIBIG := computePagIBIG(monthlySalary)

	// Pro-rate the contributions
	proratedSSS := (monthlySSS / float64(totalWorkingDays)) * float64(daysWorked)
	proratedPhilHealth := (monthlyPhilHealth / float64(totalWorkingDays)) * float64(daysWorked)
	proratedPagIBIG := (monthlyPagIBIG / float64(totalWorkingDays)) * float64(daysWorked)

	annualSSS := proratedSSS * 12
	annualPhilHealth := proratedPhilHealth * 12
	annualPagIBIG := proratedPagIBIG * 12

	taxableIncome := annualSalary - annualSSS - annualPhilHealth - annualPagIBIG

	var annualTax float64

	// 2024 Tax Table (TRAIN Law)
	switch {
	case taxableIncome <= 250000:
		annualTax = 0 // Tax exempt
	case taxableIncome <= 400000:
		annualTax = (taxableIncome - 250000) * 0.15
	case taxableIncome <= 800000:
		annualTax = 22500 + (taxableIncome-400000)*0.20
	case taxableIncome <= 2000000:
		annualTax = 102500 + (taxableIncome-800000)*0.25
	case taxableIncome <= 8000000:
		annualTax = 402500 + (taxableIncome-2000000)*0.30
	default:
		annualTax = 2202500 + (taxableIncome-8000000)*0.35
	}

	// Convert to monthly, then pro-rate based on days worked
	monthlyTax := annualTax / 12
	return math.Round(monthlyTax*100) / 100
}

// ============ API ENDPOINT - COMPUTE CONTRIBUTIONS ============
func ComputeContributions(c *gin.Context) {
	var request struct {
		BasicSalary      float64 `json:"basic_salary"`
		DaysWorked       int     `json:"days_worked"`
		TotalWorkingDays int     `json:"total_working_days"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// ✅ DEFAULT VALUES
	if request.TotalWorkingDays == 0 {
		request.TotalWorkingDays = 22
	}
	if request.DaysWorked == 0 {
		request.DaysWorked = request.TotalWorkingDays
	}

	response := ContributionResponse{
		BasicSalary: request.BasicSalary,
		SSS:         computeSSS(request.BasicSalary),
		PhilHealth:  computePhilHealth(request.BasicSalary),
		PagIBIG:     computePagIBIG(request.BasicSalary),
		Tax:         computeWithholdingTax(request.BasicSalary, request.DaysWorked, request.TotalWorkingDays),
	}

	log.Printf("💰 Computed contributions for ₱%.2f (%d/%d days): SSS=₱%.2f, PhilHealth=₱%.2f, PagIBIG=₱%.2f, Tax=₱%.2f",
		response.BasicSalary, request.DaysWorked, request.TotalWorkingDays,
		response.SSS, response.PhilHealth, response.PagIBIG, response.Tax)

	c.JSON(http.StatusOK, response)
}

// ============ GET EMPLOYEE FROM SESSION ============
func GetCurrentEmployeeSession(c *gin.Context) {
	session := sessions.Default(c)

	employeeID := session.Get("employee_id")

	if employeeID == nil {
		log.Println("❌ No employee session found")
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Message: "No employee session found. Please login again.",
		})
		return
	}

	// Get basic_salary from session directly
	basicSalary := 0.0
	if sessionSalary := session.Get("employee_basic_salary"); sessionSalary != nil {
		// Check the type of the session value
		switch v := sessionSalary.(type) {
		case float64:
			basicSalary = v
		case int:
			basicSalary = float64(v)
		case int64:
			basicSalary = float64(v)
		default:
			log.Printf("⚠️ Unexpected type for basic_salary: %T", v)
		}
	}

	employeeData := gin.H{
		"employee_id":  employeeID,
		"first_name":   session.Get("employee_first_name"),
		"last_name":    session.Get("employee_last_name"),
		"position":     session.Get("employee_position"),
		"department":   session.Get("employee_department"),
		"email":        session.Get("employee_email"),
		"contact":      session.Get("employee_contact"),
		"date_hired":   session.Get("employee_date_hired"),
		"basic_salary": basicSalary,
		"role":         session.Get("employee_role"),
		"gender":       session.Get("employee_gender"),
	}

	log.Printf("✅ Employee session data retrieved: %v - %v %v (Salary: ₱%.2f)",
		employeeData["employee_id"],
		employeeData["first_name"],
		employeeData["last_name"],
		basicSalary)

	c.JSON(http.StatusOK, employeeData)
}

// SearchEmployee searches for employee by ID
func SearchEmployee(c *gin.Context) {
	employeeID := c.Param("id")

	log.Printf("🔍 Searching for employee ID: %s", employeeID)

	var emp EmployeeInfoAPI
	query := `SELECT id, employee_id, first_name, last_name, position, department, 
	          email, contact, date_hired, basic_salary 
	          FROM employees WHERE employee_id = ?`

	err := db.QueryRow(query, employeeID).Scan(
		&emp.ID,
		&emp.EmployeeID,
		&emp.FirstName,
		&emp.LastName,
		&emp.Position,
		&emp.Department,
		&emp.Email,
		&emp.ContactNumber,
		&emp.DateHired,
		&emp.BasicSalary,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("❌ Employee not found: %s", employeeID)
			c.JSON(http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Employee not found",
			})
			return
		}
		log.Println("❌ Database error:", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Internal server error",
		})
		return
	}

	log.Printf("✅ Employee found: %s - %s %s (Salary: ₱%.2f)", emp.EmployeeID, emp.FirstName, emp.LastName, emp.BasicSalary)
	c.JSON(http.StatusOK, emp)
}

func GetJobGradeSalary(c *gin.Context) {
	grade := c.Query("grade")
	position := c.Query("position")

	var salary float64
	err := db.QueryRow(`
		SELECT basic_salary 
		FROM job_grades 
		WHERE grade = ? AND position = ?
	`, grade, position).Scan(&salary)

	if err != nil {
		c.JSON(404, gin.H{"error": "Salary not found"})
		return
	}

	c.JSON(200, gin.H{
		"grade":        grade,
		"position":     position,
		"basic_salary": salary,
	})
}

func SavePayroll(c *gin.Context) {
	var payroll Payroll
	if err := c.ShouldBindJSON(&payroll); err != nil {
		log.Printf("❌ Invalid JSON data: %v", err)
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request data",
		})
		return
	}

	// Validate employee exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM employees WHERE employee_id = ?)",
		payroll.EmployeeID).Scan(&exists)
	if err != nil || !exists {
		log.Printf("❌ Employee not found: %s", payroll.EmployeeID)
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Employee not found",
		})
		return
	}

	// ========================================
	// ✅ COMPREHENSIVE PERIOD VALIDATION
	// ========================================

	periodFrom, err1 := time.Parse("2006-01-02", payroll.PeriodFrom)
	periodTo, err2 := time.Parse("2006-01-02", payroll.PeriodTo)

	if err1 != nil || err2 != nil {
		log.Printf("❌ Invalid period dates")
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid period dates format",
		})
		return
	}

	now := time.Now()
	currentDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	currentYear := now.Year()
	currentMonth := now.Month()
	currentDay := now.Day()

	periodDays := int(periodTo.Sub(periodFrom).Hours()/24) + 1
	periodYear := periodFrom.Year()
	periodMonth := periodFrom.Month()
	monthName := periodMonth.String()

	// VALIDATION 1: Period From cannot be after Period To
	if periodFrom.After(periodTo) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "❌ INVALID PERIOD! 'Period From' date cannot be later than 'Period To' date.",
		})
		return
	}

	// VALIDATION 2: Both dates must be in the same month
	if periodFrom.Month() != periodTo.Month() || periodFrom.Year() != periodTo.Year() {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ INVALID PERIOD! Both 'Period From' and 'Period To' must be within the same month. You selected dates from %s to %s.",
				periodFrom.Format("January 2006"), periodTo.Format("January 2006")),
		})
		return
	}

	// VALIDATION 3: Period must be within current month
	if periodFrom.Year() != currentYear || periodFrom.Month() != currentMonth {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ INVALID PERIOD! You can only submit payroll for the current month (%s %d). You selected %s %d.",
				currentMonth.String(), currentYear, monthName, periodYear),
		})
		return
	}

	// VALIDATION 4: Cannot submit payroll for future dates
	if periodTo.After(currentDate) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ FUTURE PERIOD NOT ALLOWED! Today is %s %d, %d. You cannot submit payroll for future dates. Your period ends on %s %d, %d. Please wait until the period is complete or select a period up to today only.",
				currentMonth.String(), currentDay, currentYear,
				periodTo.Month().String(), periodTo.Day(), periodTo.Year()),
		})
		return
	}

	// VALIDATION 5: Days Worked must match period length EXACTLY
	if payroll.DaysWorked != periodDays {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ DAYS MISMATCH! Your 'Days Worked' (%d days) must exactly match your selected period length (%d days). Period: %s to %s = %d days.",
				payroll.DaysWorked, periodDays, payroll.PeriodFrom, payroll.PeriodTo, periodDays),
		})
		return
	}

	// VALIDATION 6: Days Worked cannot exceed Total Working Days
	if payroll.DaysWorked > payroll.TotalWorkingDays {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ INVALID INPUT! Days Worked (%d) cannot exceed Total Working Days (%d) in this month.",
				payroll.DaysWorked, payroll.TotalWorkingDays),
		})
		return
	}

	// VALIDATION 7: Days Worked cannot be zero or negative
	if payroll.DaysWorked <= 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "❌ INVALID INPUT! Days Worked must be at least 1 day.",
		})
		return
	}

	// VALIDATION 8: Period cannot be in the past (more than current month)
	if periodYear < currentYear || (periodYear == currentYear && periodMonth < currentMonth) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ PAST PERIOD NOT ALLOWED! You can only submit payroll for the current month (%s %d). You selected %s %d.",
				currentMonth.String(), currentYear, monthName, periodYear),
		})
		return
	}

	// ✅ HOLIDAY DATE VALIDATION
	if payroll.RegularHolidayDate != "" {
		holidayDate, err := time.Parse("2006-01-02", payroll.RegularHolidayDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Invalid regular holiday date format",
			})
			return
		}

		if holidayDate.Month() != currentMonth || holidayDate.Year() != currentYear {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: fmt.Sprintf("Regular holiday date must be in current month (%s %d)", currentMonth.String(), currentYear),
			})
			return
		}

		var holidayName string
		err = db.QueryRow("SELECT holiday_name FROM holidays WHERE holiday_date = ?", payroll.RegularHolidayDate).Scan(&holidayName)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: fmt.Sprintf("❌ Date %s is not a registered holiday", payroll.RegularHolidayDate),
			})
			return
		}
		log.Printf("✅ Regular holiday validated: %s - %s", payroll.RegularHolidayDate, holidayName)
	}

	if payroll.SpecialHolidayDate != "" {
		holidayDate, err := time.Parse("2006-01-02", payroll.SpecialHolidayDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: "Invalid special holiday date format",
			})
			return
		}

		if holidayDate.Month() != currentMonth || holidayDate.Year() != currentYear {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: fmt.Sprintf("Special holiday date must be in current month (%s %d)", currentMonth.String(), currentYear),
			})
			return
		}

		var holidayName string
		err = db.QueryRow("SELECT holiday_name FROM holidays WHERE holiday_date = ?", payroll.SpecialHolidayDate).Scan(&holidayName)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Message: fmt.Sprintf("❌ Date %s is not a registered holiday", payroll.SpecialHolidayDate),
			})
			return
		}
		log.Printf("✅ Special holiday validated: %s - %s", payroll.SpecialHolidayDate, holidayName)
	}

	log.Printf("✅ Validation passed: Period=%d days, Worked=%d days, Month Total=%d days, Period: %s to %s",
		periodDays, payroll.DaysWorked, payroll.TotalWorkingDays, payroll.PeriodFrom, payroll.PeriodTo)

	// ✅ Calculate gross pay INCLUDING holiday pay
	payroll.GrossPay = payroll.ProratedSalary + payroll.Transportation +
		payroll.MealAllowance + payroll.Communication + payroll.OtherAllowance +
		payroll.RegularHolidayPay + payroll.SpecialHolidayPay

	payroll.TotalDeductions = payroll.SSS + payroll.PhilHealth +
		payroll.PagIBIG + payroll.Tax + payroll.CashAdvance + payroll.OtherDeductions

	payroll.NetPay = payroll.GrossPay - payroll.TotalDeductions

	// ========================================
	// ✅ DETERMINE APPROVER ROLE BASED ON EMPLOYEE ROLE
	// Supervisor employees → Manager approves
	// Regular employees   → Supervisor approves
	// ========================================
	var employeeRole string
	err = db.QueryRow("SELECT COALESCE(role, 'Employee') FROM employees WHERE employee_id = ?",
		payroll.EmployeeID).Scan(&employeeRole)
	if err != nil {
		log.Printf("⚠️ Could not get employee role, defaulting to Supervisor approver: %v", err)
		employeeRole = "Employee"
	}

	approverRole := "Supervisor" // default — regular employees go to Supervisor
	if strings.EqualFold(employeeRole, "Supervisor") {
		approverRole = "Manager" // Supervisor employees go to Manager
	}

	log.Printf("👤 Employee %s role: '%s' → Approver: '%s'", payroll.EmployeeID, employeeRole, approverRole)

	log.Printf("💰 Saving payroll for employee: %s (Days: %d/%d, Net Pay: ₱%.2f, Approver: %s)",
		payroll.EmployeeID, payroll.DaysWorked, payroll.TotalWorkingDays, payroll.NetPay, approverRole)

	// ========================================
	// ✅ SAVE TO DATABASE
	// ========================================
	query := `INSERT INTO payroll (
    employee_id, basic_salary, total_working_days, days_worked, prorated_salary,
    regular_holiday_date, special_holiday_date,
    regular_holiday_days, special_holiday_days, regular_holiday_pay, special_holiday_pay,
    sick_leave, vacation_leave, maternity_leave, paternity_leave, other_leave,
    transportation, meal_allowance, communication, other_allowance,
    sss, philhealth, pagibig, tax, cash_advance, other_deductions,
    gross_pay, total_deductions, net_pay, period_from, period_to,
    status, approver_role, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`

	result, err := db.Exec(query,
		payroll.EmployeeID,         // 1
		payroll.BasicSalary,        // 2
		payroll.TotalWorkingDays,   // 3
		payroll.DaysWorked,         // 4
		payroll.ProratedSalary,     // 5
		payroll.RegularHolidayDate, // 6
		payroll.SpecialHolidayDate, // 7
		payroll.RegularHolidayDays, // 8
		payroll.SpecialHolidayDays, // 9
		payroll.RegularHolidayPay,  // 10
		payroll.SpecialHolidayPay,  // 11
		payroll.SickLeave,          // 12
		payroll.VacationLeave,      // 13
		payroll.MaternityLeave,     // 14
		payroll.PaternityLeave,     // 15
		payroll.OtherLeave,         // 16
		payroll.Transportation,     // 17
		payroll.MealAllowance,      // 18
		payroll.Communication,      // 19
		payroll.OtherAllowance,     // 20
		payroll.SSS,                // 21
		payroll.PhilHealth,         // 22
		payroll.PagIBIG,            // 23
		payroll.Tax,                // 24
		payroll.CashAdvance,        // 25
		payroll.OtherDeductions,    // 26
		payroll.GrossPay,           // 27
		payroll.TotalDeductions,    // 28
		payroll.NetPay,             // 29
		payroll.PeriodFrom,         // 30
		payroll.PeriodTo,           // 31
		"Pending",                  // 32 - status
		approverRole,               // 33 - approver_role ✅ BAGO
	)

	if err != nil {
		log.Println("❌ Insert error:", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to save payroll",
		})
		return
	}

	lastID, _ := result.LastInsertId()
	payroll.ID = int(lastID)

	log.Printf("✅ Payroll saved successfully - ID: %d, Approver: %s", payroll.ID, approverRole)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Payroll saved successfully. Submitted to %s for approval.", approverRole),
		Data:    payroll,
	})
}

func RequestLeave(c *gin.Context) {
	var leave Leave

	if err := c.ShouldBindJSON(&leave); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid leave request",
		})
		return
	}

	// ✅ STEP 1: GET EMPLOYEE GENDER FROM DATABASE
	var employeeGender string
	err := db.QueryRow(`
		SELECT gender FROM employees WHERE employee_id = ?
	`, leave.EmployeeID).Scan(&employeeGender)

	if err != nil {
		log.Printf("❌ Failed to get employee gender: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to validate employee information",
		})
		return
	}

	log.Printf("👤 Employee %s gender: %s, requesting %s", leave.EmployeeID, employeeGender, leave.LeaveType)

	// ✅ STEP 2: VALIDATE GENDER-SPECIFIC LEAVE TYPES
	if leave.LeaveType == "Maternity Leave" && employeeGender != "Female" {
		log.Printf("❌ Male employee attempting maternity leave: %s", leave.EmployeeID)
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "❌ Maternity Leave is only available for female employees. As a male employee, you can apply for Paternity Leave instead.",
		})
		return
	}

	if leave.LeaveType == "Paternity Leave" && employeeGender != "Male" {
		log.Printf("❌ Female employee attempting paternity leave: %s", leave.EmployeeID)
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "❌ Paternity Leave is only available for male employees. As a female employee, you can apply for Maternity Leave instead.",
		})
		return
	}

	// 🔢 STEP 3: COMPUTE LEAVE DAYS
	layout := "2006-01-02"
	start, err1 := time.Parse(layout, leave.LeaveFrom)
	end, err2 := time.Parse(layout, leave.LeaveTo)

	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid leave dates",
		})
		return
	}

	leave.LeaveDays = int(end.Sub(start).Hours()/24) + 1

	if leave.LeaveDays <= 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid leave duration",
		})
		return
	}

	log.Printf("🔍 DEBUG - Leave Type: '%s', Days: %d", leave.LeaveType, leave.LeaveDays)

	// ✅ STEP 4: CHECK LEAVE BALANCE
	year := time.Now().Year()
	var leaveUsed int

	db.QueryRow(`
		SELECT COALESCE(SUM(leave_days), 0) 
		FROM leave_requests 
		WHERE employee_id = ? 
		AND leave_type = ? 
		AND status IN ('Approved', 'Pending')
		AND YEAR(leave_from) = ?
	`, leave.EmployeeID, leave.LeaveType, year).Scan(&leaveUsed)

	// Determine max allocation
	maxAllocation := 15 // default for Sick/Vacation
	if leave.LeaveType == "Emergency Leave" {
		maxAllocation = 5
	} else if leave.LeaveType == "Maternity Leave" {
		maxAllocation = 105
	} else if leave.LeaveType == "Paternity Leave" {
		maxAllocation = 7
	}

	remaining := maxAllocation - leaveUsed

	log.Printf("📊 Leave check: Type=%s, Used=%d, Max=%d, Remaining=%d, Requested=%d",
		leave.LeaveType, leaveUsed, maxAllocation, remaining, leave.LeaveDays)

	// ✅ CHECK IF NO CREDITS REMAINING
	if remaining <= 0 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ No %s credits remaining! You have used all %d days for this year. Your credits will reset on January 1, %d.",
				leave.LeaveType, maxAllocation, year+1),
		})
		return
	}

	// ✅ CHECK IF REQUESTED EXCEEDS REMAINING
	if leave.LeaveDays > remaining {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: fmt.Sprintf("❌ Insufficient %s credits! You only have %d day(s) remaining but requested %d day(s). Credits will reset on January 1, %d.",
				leave.LeaveType, remaining, leave.LeaveDays, year+1),
		})
		return
	}

	// 🔥 STEP 5: DETERMINE APPROVER
	if leave.LeaveDays <= 2 {
		leave.ApproverRole = "Supervisor"
		leave.ApproverDashboard = "supervisor_dashboard.html"
	} else {
		leave.ApproverRole = "Manager"
		leave.ApproverDashboard = "manager_dashboard.html"
	}

	leave.Status = "Pending"

	// 💾 STEP 6: SAVE TO DATABASE
	_, err = db.Exec(`
		INSERT INTO leave_requests
		(employee_id, leave_type, leave_from, leave_to, leave_days, reason, approver_role, approver_dashboard, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`,
		leave.EmployeeID,
		leave.LeaveType,
		leave.LeaveFrom,
		leave.LeaveTo,
		leave.LeaveDays,
		leave.Reason,
		leave.ApproverRole,
		leave.ApproverDashboard,
		leave.Status,
	)

	if err != nil {
		log.Println("❌ Leave insert error:", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to submit leave",
		})
		return
	}

	log.Printf("✅ Leave request saved: %s - %d days", leave.LeaveType, leave.LeaveDays)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Leave request submitted successfully",
		Data: gin.H{
			"leave_days": leave.LeaveDays,
			"approver":   leave.ApproverRole,
			"redirect":   leave.ApproverDashboard,
		},
	})
}

// ✅ FINAL FIX - PALITAN MO YUNG BUONG GetPayrollHistory FUNCTION
// ✅ FIX #1: c.Param("id") instead of c.Param("employee_id")
// ✅ FIX #2: Scan created_at as string first, then parse to time.Time

func GetPayrollHistory(c *gin.Context) {
	// ✅ FIX: Use "id" to match the route parameter ":id"
	employeeID := c.Param("id")

	if employeeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Employee ID is required",
		})
		return
	}

	log.Printf("📊 Fetching payroll history for employee: %s", employeeID)

	query := `
		SELECT 
			id,
			employee_id,
			basic_salary,
			total_working_days,
			days_worked,
			prorated_salary,

			COALESCE(regular_holiday_date, ''),
			COALESCE(special_holiday_date, ''),
			COALESCE(regular_holiday_days, 0),
			COALESCE(special_holiday_days, 0),
			COALESCE(regular_holiday_pay, 0),
			COALESCE(special_holiday_pay, 0),

			COALESCE(sick_leave, 0),
			COALESCE(vacation_leave, 0),
			COALESCE(maternity_leave, 0),
			COALESCE(paternity_leave, 0),
			COALESCE(other_leave, 0),

			COALESCE(transportation, 0),
			COALESCE(meal_allowance, 0),
			COALESCE(communication, 0),
			COALESCE(other_allowance, 0),

			COALESCE(sss, 0),
			COALESCE(philhealth, 0),
			COALESCE(pagibig, 0),
			COALESCE(tax, 0),
			COALESCE(cash_advance, 0),
			COALESCE(other_deductions, 0),

			COALESCE(gross_pay, 0),
			COALESCE(total_deductions, 0),
			COALESCE(net_pay, 0),

			period_from,
			period_to,
			IFNULL(status, 'Pending'),
			IFNULL(supervisor_notes, ''),
			DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
		FROM payroll
		WHERE employee_id = ?
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query, employeeID)
	if err != nil {
		log.Printf("❌ Database query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error",
			"error":   err.Error(),
		})
		return
	}
	defer rows.Close()

	type PayrollWithStatus struct {
		ID               int     `json:"id"`
		EmployeeID       string  `json:"employee_id"`
		BasicSalary      float64 `json:"basic_salary"`
		TotalWorkingDays int     `json:"total_working_days"`
		DaysWorked       int     `json:"days_worked"`
		ProratedSalary   float64 `json:"prorated_salary"`

		RegularHolidayDate string  `json:"regular_holiday_date"`
		SpecialHolidayDate string  `json:"special_holiday_date"`
		RegularHolidayDays int     `json:"regular_holiday_days"`
		SpecialHolidayDays int     `json:"special_holiday_days"`
		RegularHolidayPay  float64 `json:"regular_holiday_pay"`
		SpecialHolidayPay  float64 `json:"special_holiday_pay"`

		SickLeave      float64 `json:"sick_leave"`
		VacationLeave  float64 `json:"vacation_leave"`
		MaternityLeave float64 `json:"maternity_leave"`
		PaternityLeave float64 `json:"paternity_leave"`
		OtherLeave     float64 `json:"other_leave"`

		Transportation float64 `json:"transportation"`
		MealAllowance  float64 `json:"meal_allowance"`
		Communication  float64 `json:"communication"`
		OtherAllowance float64 `json:"other_allowance"`

		SSS             float64 `json:"sss"`
		PhilHealth      float64 `json:"philhealth"`
		PagIBIG         float64 `json:"pagibig"`
		Tax             float64 `json:"tax"`
		CashAdvance     float64 `json:"cash_advance"`
		OtherDeductions float64 `json:"other_deductions"`

		GrossPay        float64 `json:"gross_pay"`
		TotalDeductions float64 `json:"total_deductions"`
		NetPay          float64 `json:"net_pay"`

		PeriodFrom      string    `json:"period_from"`
		PeriodTo        string    `json:"period_to"`
		Status          string    `json:"status"`
		SupervisorNotes string    `json:"supervisor_notes"`
		CreatedAt       time.Time `json:"created_at"`
	}

	history := []PayrollWithStatus{}

	for rows.Next() {
		var p PayrollWithStatus
		var createdAtStr string // ✅ FIX: Scan as string first

		err := rows.Scan(
			&p.ID,
			&p.EmployeeID,
			&p.BasicSalary,
			&p.TotalWorkingDays,
			&p.DaysWorked,
			&p.ProratedSalary,

			&p.RegularHolidayDate,
			&p.SpecialHolidayDate,
			&p.RegularHolidayDays,
			&p.SpecialHolidayDays,
			&p.RegularHolidayPay,
			&p.SpecialHolidayPay,

			&p.SickLeave,
			&p.VacationLeave,
			&p.MaternityLeave,
			&p.PaternityLeave,
			&p.OtherLeave,

			&p.Transportation,
			&p.MealAllowance,
			&p.Communication,
			&p.OtherAllowance,

			&p.SSS,
			&p.PhilHealth,
			&p.PagIBIG,
			&p.Tax,
			&p.CashAdvance,
			&p.OtherDeductions,

			&p.GrossPay,
			&p.TotalDeductions,
			&p.NetPay,

			&p.PeriodFrom,
			&p.PeriodTo,
			&p.Status,
			&p.SupervisorNotes,
			&createdAtStr, // ✅ FIX: Scan to string
		)

		if err != nil {
			log.Printf("❌ Row scan error: %v", err)
			continue
		}

		// ✅ FIX: Parse string to time.Time
		layout := "2006-01-02 15:04:05"
		p.CreatedAt, _ = time.Parse(layout, createdAtStr)

		history = append(history, p)
	}

	if err := rows.Err(); err != nil {
		log.Printf("❌ Rows error: %v", err)
	}

	log.Printf("✅ Returning %d payroll records for %s", len(history), employeeID)
	c.JSON(http.StatusOK, history)
}

// PayrollApprovalRequest for supervisor decision
type PayrollApprovalRequest struct {
	PayrollID int    `json:"payroll_id"`
	Status    string `json:"status"` // "Approved" or "Rejected"
	Notes     string `json:"notes"`
}

func GetPendingPayrolls(c *gin.Context) {
	log.Println("📊 Fetching pending payrolls for supervisor review")

	query := `
		SELECT 
			p.id, p.employee_id, p.basic_salary, p.total_working_days, 
			p.days_worked, p.prorated_salary, p.transportation, p.meal_allowance,
			p.communication, p.other_allowance, p.sss, p.philhealth, p.pagibig,
			p.tax, p.cash_advance, p.other_deductions, p.gross_pay,
			p.total_deductions, p.net_pay, p.period_from, p.period_to,
			p.status, p.supervisor_notes,
			DATE_FORMAT(p.created_at, '%Y-%m-%d %H:%i:%s') as created_at,
			e.first_name, e.last_name, e.position, e.department
		FROM payroll p
		JOIN employees e ON p.employee_id = e.employee_id
		WHERE p.status = 'Pending' 
		AND COALESCE(p.approver_role, 'Supervisor') = 'Supervisor'
		ORDER BY p.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to fetch payrolls",
		})
		return
	}
	defer rows.Close()

	type PayrollWithEmployee struct {
		Payroll
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		Position        string `json:"position"`
		Department      string `json:"department"`
		Status          string `json:"status"`
		SupervisorNotes string `json:"supervisor_notes"`
	}

	var payrolls []PayrollWithEmployee
	for rows.Next() {
		var p PayrollWithEmployee
		var createdAtStr string
		var notes sql.NullString

		err := rows.Scan(
			&p.ID, &p.EmployeeID, &p.BasicSalary, &p.TotalWorkingDays,
			&p.DaysWorked, &p.ProratedSalary, &p.Transportation, &p.MealAllowance,
			&p.Communication, &p.OtherAllowance, &p.SSS, &p.PhilHealth, &p.PagIBIG,
			&p.Tax, &p.CashAdvance, &p.OtherDeductions, &p.GrossPay,
			&p.TotalDeductions, &p.NetPay, &p.PeriodFrom, &p.PeriodTo,
			&p.Status, &notes, &createdAtStr,
			&p.FirstName, &p.LastName, &p.Position, &p.Department,
		)

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		if notes.Valid {
			p.SupervisorNotes = notes.String
		}

		layout := "2006-01-02 15:04:05"
		p.CreatedAt, _ = time.Parse(layout, createdAtStr)

		payrolls = append(payrolls, p)
	}

	log.Printf("✅ Found %d pending payrolls for Supervisor", len(payrolls))
	c.JSON(http.StatusOK, payrolls)
}

// GetLeaveHistory retrieves leave history for an employee
func GetLeaveHistory(c *gin.Context) {
	employeeID := c.Param("id")

	log.Printf("📋 Fetching leave history for employee: %s", employeeID)

	query := `SELECT 
		id, employee_id, leave_type, leave_from, leave_to, leave_days, 
		reason, approver_role, approver_dashboard, status,
		DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
		FROM leave_requests 
		WHERE employee_id = ? 
		ORDER BY created_at DESC`

	rows, err := db.Query(query, employeeID)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve leave history",
		})
		return
	}
	defer rows.Close()

	type LeaveHistory struct {
		ID                int    `json:"id"`
		EmployeeID        string `json:"employee_id"`
		LeaveType         string `json:"leave_type"`
		LeaveFrom         string `json:"leave_from"`
		LeaveTo           string `json:"leave_to"`
		LeaveDays         int    `json:"leave_days"`
		Reason            string `json:"reason"`
		ApproverRole      string `json:"approver_role"`
		ApproverDashboard string `json:"approver_dashboard"`
		Status            string `json:"status"`
		CreatedAt         string `json:"created_at"`
	}

	var history []LeaveHistory
	for rows.Next() {
		var h LeaveHistory
		err := rows.Scan(
			&h.ID, &h.EmployeeID, &h.LeaveType, &h.LeaveFrom, &h.LeaveTo,
			&h.LeaveDays, &h.Reason, &h.ApproverRole, &h.ApproverDashboard,
			&h.Status, &h.CreatedAt,
		)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}
		history = append(history, h)
	}

	log.Printf("✅ Found %d leave records for employee: %s", len(history), employeeID)
	c.JSON(http.StatusOK, history)
}

// ApproveOrRejectPayroll - Supervisor decision
func ApproveOrRejectPayroll(c *gin.Context) {
	var request PayrollApprovalRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request",
		})
		return
	}

	// Get supervisor info from session
	session := sessions.Default(c)
	reviewerName := session.Get("employee_first_name")
	if reviewerName == nil {
		reviewerName = "Supervisor"
	}

	log.Printf("📝 Payroll #%d being reviewed by %s - Status: %s",
		request.PayrollID, reviewerName, request.Status)

	query := `
		UPDATE payroll 
		SET status = ?, 
		    supervisor_notes = ?, 
		    reviewed_by = ?,
		    reviewed_at = NOW()
		WHERE id = ?
	`

	_, err := db.Exec(query, request.Status, request.Notes, reviewerName, request.PayrollID)
	if err != nil {
		log.Printf("❌ Update error: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to update payroll",
		})
		return
	}

	log.Printf("✅ Payroll #%d %s successfully", request.PayrollID, request.Status)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Payroll %s successfully", request.Status),
	})
}

func GetPendingLeaves(c *gin.Context) {
	role := c.Param("role") // "Supervisor" or "Manager"

	// ✅ MAKE IT CASE-INSENSITIVE
	roleCapitalized := strings.Title(strings.ToLower(role))

	log.Printf("📋 Fetching pending leaves for role: %s (original: %s)", roleCapitalized, role)

	query := `SELECT id, employee_id, leave_type, leave_from, leave_to, 
          leave_days, reason, status, 
          DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
          FROM leave_requests 
          WHERE approver_role = ? AND status = 'Pending'
          ORDER BY created_at DESC`

	rows, err := db.Query(query, roleCapitalized)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to load leaves",
			"details": err.Error(),
		})
		return
	}
	defer rows.Close()

	var leaves []Leave
	for rows.Next() {
		var leave Leave
		var createdAtStr string // ← READ AS STRING FIRST

		err := rows.Scan(&leave.ID, &leave.EmployeeID, &leave.LeaveType,
			&leave.LeaveFrom, &leave.LeaveTo, &leave.LeaveDays,
			&leave.Reason, &leave.Status, &createdAtStr) // ← SCAN TO STRING

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		// ✅ CONVERT STRING TO TIME.TIME
		layout := "2006-01-02 15:04:05"
		leave.CreatedAt, _ = time.Parse(layout, createdAtStr)

		leaves = append(leaves, leave)
	}

	log.Printf("✅ Found %d pending leaves for %s", len(leaves), roleCapitalized)

	c.JSON(http.StatusOK, leaves)
}

func UpdateLeaveDecision(c *gin.Context) {
	var req struct {
		LeaveID int    `json:"leave_id"`
		Status  string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	_, err := db.Exec("UPDATE leave_requests SET status = ? WHERE id = ?",
		req.Status, req.LeaveID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to update"})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

// ListAllEmployees returns all employee IDs and names
func ListAllEmployees(c *gin.Context) {
	log.Println("📝 Fetching all employees list")

	rows, err := db.Query(`SELECT employee_id, first_name, last_name, position, department 
	                        FROM employees 
	                        ORDER BY employee_id`)
	if err != nil {
		log.Println("❌ Database error:", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "Failed to fetch employees",
		})
		return
	}
	defer rows.Close()

	type EmployeeListItem struct {
		EmployeeID string `json:"employee_id"`
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Position   string `json:"position"`
		Department string `json:"department"`
	}

	var employees []EmployeeListItem
	for rows.Next() {
		var emp EmployeeListItem
		if err := rows.Scan(&emp.EmployeeID, &emp.FirstName, &emp.LastName,
			&emp.Position, &emp.Department); err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}
		employees = append(employees, emp)
	}

	log.Printf("✅ Found %d employees", len(employees))
	c.JSON(http.StatusOK, employees)
}

/* ===========================
   MANAGER API FUNCTIONS
=========================== */

func GetPendingSupervisorPayrolls(c *gin.Context) {
	log.Println("📊 Fetching pending supervisor payrolls for manager review")

	query := `
		SELECT 
			p.id, p.employee_id, e.first_name, e.last_name, e.position, e.department,
			p.period_from, p.period_to, p.days_worked, p.total_working_days,
			p.basic_salary, p.prorated_salary, p.gross_pay, p.total_deductions, p.net_pay,
			COALESCE(p.transportation, 0) as transportation,
			COALESCE(p.meal_allowance, 0) as meal_allowance,
			COALESCE(p.communication, 0) as communication,
			COALESCE(p.other_allowance, 0) as other_allowance,
			COALESCE(p.sss, 0) as sss,
			COALESCE(p.philhealth, 0) as philhealth,
			COALESCE(p.pagibig, 0) as pagibig,
			COALESCE(p.tax, 0) as tax,
			COALESCE(p.cash_advance, 0) as cash_advance,
			COALESCE(p.other_deductions, 0) as other_deductions,
			p.status,
			DATE_FORMAT(p.created_at, '%Y-%m-%d') as submitted_date
		FROM payroll p
		JOIN employees e ON p.employee_id = e.employee_id
		WHERE p.status = 'Pending' 
		AND p.approver_role = 'Manager'
		ORDER BY p.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch payrolls",
		})
		return
	}
	defer rows.Close()

	type ManagerPayrollView struct {
		ID               int     `json:"id"`
		EmployeeID       string  `json:"employee_id"`
		FirstName        string  `json:"first_name"`
		LastName         string  `json:"last_name"`
		Position         string  `json:"position"`
		Department       string  `json:"department"`
		PeriodFrom       string  `json:"period_from"`
		PeriodTo         string  `json:"period_to"`
		DaysWorked       int     `json:"days_worked"`
		TotalWorkingDays int     `json:"total_working_days"`
		BasicSalary      float64 `json:"basic_salary"`
		ProratedSalary   float64 `json:"prorated_salary"`
		Transportation   float64 `json:"transportation"`
		MealAllowance    float64 `json:"meal_allowance"`
		Communication    float64 `json:"communication"`
		OtherAllowance   float64 `json:"other_allowance"`
		SSS              float64 `json:"sss"`
		PhilHealth       float64 `json:"philhealth"`
		PagIBIG          float64 `json:"pagibig"`
		Tax              float64 `json:"tax"`
		CashAdvance      float64 `json:"cash_advance"`
		OtherDeductions  float64 `json:"other_deductions"`
		GrossPay         float64 `json:"gross_pay"`
		TotalDeductions  float64 `json:"total_deductions"`
		NetPay           float64 `json:"net_pay"`
		Status           string  `json:"status"`
		SubmittedDate    string  `json:"submitted_date"`
	}

	var payrolls []ManagerPayrollView
	for rows.Next() {
		var p ManagerPayrollView
		err := rows.Scan(
			&p.ID, &p.EmployeeID, &p.FirstName, &p.LastName, &p.Position, &p.Department,
			&p.PeriodFrom, &p.PeriodTo, &p.DaysWorked, &p.TotalWorkingDays,
			&p.BasicSalary, &p.ProratedSalary, &p.GrossPay, &p.TotalDeductions, &p.NetPay,
			&p.Transportation, &p.MealAllowance, &p.Communication, &p.OtherAllowance,
			&p.SSS, &p.PhilHealth, &p.PagIBIG, &p.Tax, &p.CashAdvance, &p.OtherDeductions,
			&p.Status, &p.SubmittedDate,
		)

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		payrolls = append(payrolls, p)
	}

	log.Printf("✅ Found %d pending Manager payrolls", len(payrolls))
	c.JSON(http.StatusOK, payrolls)
}

// ManagerApprovePayroll - Manager decision on payroll
func ManagerApprovePayroll(c *gin.Context) {
	var request struct {
		PayrollID int    `json:"payroll_id"`
		Status    string `json:"status"` // "Approved" or "Rejected"
		Notes     string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	// Validate status
	if request.Status != "Approved" && request.Status != "Rejected" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Status must be 'Approved' or 'Rejected'",
		})
		return
	}

	// Require notes for rejection
	if request.Status == "Rejected" && strings.TrimSpace(request.Notes) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Notes are required when rejecting a payroll",
		})
		return
	}

	// Get manager info from session
	session := sessions.Default(c)
	managerName := session.Get("employee_first_name")
	if managerName == nil {
		managerName = "Manager"
	}

	log.Printf("📝 Manager %s reviewing payroll #%d - Status: %s",
		managerName, request.PayrollID, request.Status)

	// Update payroll status
	query := `
		UPDATE payroll 
		SET status = ?, 
		    manager_notes = ?, 
		    manager_reviewed_by = ?,
		    manager_reviewed_at = NOW()
		WHERE id = ?
	`

	result, err := db.Exec(query, request.Status, request.Notes, managerName, request.PayrollID)
	if err != nil {
		log.Printf("❌ Update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update payroll",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Payroll not found",
		})
		return
	}

	log.Printf("✅ Payroll #%d %s by manager successfully", request.PayrollID, request.Status)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Payroll %s successfully", strings.ToLower(request.Status)),
	})
}

// GetPendingManagerLeaves - For Manager Dashboard Leave Tab
func GetPendingManagerLeaves(c *gin.Context) {
	log.Println("📋 Fetching pending manager leave requests")

	query := `
		SELECT id, employee_id, leave_type, leave_from, leave_to, 
		       leave_days, reason, status,
		       DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
		FROM leave_requests 
		WHERE approver_role = 'Manager' AND status = 'Pending'
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to load leave requests",
		})
		return
	}
	defer rows.Close()

	var leaves []Leave
	for rows.Next() {
		var leave Leave
		var createdAtStr string

		err := rows.Scan(
			&leave.ID, &leave.EmployeeID, &leave.LeaveType,
			&leave.LeaveFrom, &leave.LeaveTo, &leave.LeaveDays,
			&leave.Reason, &leave.Status, &createdAtStr,
		)

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		layout := "2006-01-02 15:04:05"
		leave.CreatedAt, _ = time.Parse(layout, createdAtStr)

		leaves = append(leaves, leave)
	}

	log.Printf("✅ Found %d pending manager leaves", len(leaves))
	c.JSON(http.StatusOK, leaves)
}

// ManagerLeaveDecision - Manager approve/reject leave
func ManagerLeaveDecision(c *gin.Context) {
	var req struct {
		LeaveID int    `json:"leave_id"`
		Status  string `json:"status"` // "Approved" or "Rejected"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	log.Printf("📝 Manager deciding on leave #%d - Status: %s", req.LeaveID, req.Status)

	_, err := db.Exec(`
		UPDATE leave_requests 
		SET status = ?, reviewed_at = NOW()
		WHERE id = ?
	`, req.Status, req.LeaveID)

	if err != nil {
		log.Printf("❌ Update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update leave",
		})
		return
	}

	log.Printf("✅ Leave #%d %s by manager", req.LeaveID, req.Status)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Leave %s successfully", strings.ToLower(req.Status)),
	})
}

func compareFaceImages(registeredDescriptorJSON, capturedDescriptorJSON string) float64 {
	// Parse registered face descriptor
	var registeredDescriptor []float64
	if err := json.Unmarshal([]byte(registeredDescriptorJSON), &registeredDescriptor); err != nil {
		log.Printf("❌ Failed to parse registered descriptor: %v", err)
		return 0
	}

	// Parse captured face descriptor
	var capturedDescriptor []float64
	if err := json.Unmarshal([]byte(capturedDescriptorJSON), &capturedDescriptor); err != nil {
		log.Printf("❌ Failed to parse captured descriptor: %v", err)
		return 0
	}

	// Validate descriptor lengths
	if len(registeredDescriptor) != 128 || len(capturedDescriptor) != 128 {
		log.Printf("❌ Invalid descriptor lengths: registered=%d, captured=%d",
			len(registeredDescriptor), len(capturedDescriptor))
		return 0
	}

	// Calculate Euclidean distance
	var sum float64
	for i := 0; i < 128; i++ {
		diff := registeredDescriptor[i] - capturedDescriptor[i]
		sum += diff * diff
	}
	distance := math.Sqrt(sum)

	// Convert distance to similarity percentage
	// ✅ ADJUSTED: More lenient threshold (0.8 instead of 0.6)
	similarity := math.Max(0, (1-(distance/0.8))*100)

	log.Printf("🔍 Face comparison: Distance=%.4f, Similarity=%.2f%%", distance, similarity)

	return similarity
}

// ✅ PALITAN MO YUNG CheckAttendance FUNCTION SA employee.go

// ✅ PALITAN MO YUNG CheckAttendance FUNCTION SA employee.go

func CheckAttendance(c *gin.Context) {
	var req struct {
		Action         string    `json:"action"`
		FaceImage      string    `json:"face_image"`
		FaceDescriptor []float64 `json:"face_descriptor"`
		DailyReport    string    `json:"daily_report"` // ✅ NEW FIELD
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	session := sessions.Default(c)
	employeeID := session.Get("employee_id")

	if employeeID == nil {
		log.Println("❌ No employee session")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
		return
	}

	log.Printf("📸 Processing %s for employee: %s", req.Action, employeeID)

	// ✅ GET REGISTERED FACE DESCRIPTOR FROM DATABASE
	var registeredFaceData sql.NullString
	err := db.QueryRow(`
		SELECT face_data FROM employees WHERE employee_id = ?
	`, employeeID).Scan(&registeredFaceData)

	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error. Please try again.",
		})
		return
	}

	if !registeredFaceData.Valid || registeredFaceData.String == "" {
		log.Printf("❌ No registered face for employee: %s", employeeID)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "❌ No registered face found. Please contact HR to register your face.",
		})
		return
	}

	// ✅ VALIDATE CAPTURED FACE DESCRIPTOR
	if len(req.FaceDescriptor) != 128 {
		log.Printf("❌ Invalid face descriptor length: %d", len(req.FaceDescriptor))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "❌ Invalid face data. Please try again with better lighting.",
		})
		return
	}

	// ✅ Convert captured descriptor to JSON string for comparison
	capturedDescriptorJSON, err := json.Marshal(req.FaceDescriptor)
	if err != nil {
		log.Printf("❌ Failed to marshal descriptor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Internal error",
		})
		return
	}

	// ✅ COMPARE FACES
	similarity := compareFaceImages(registeredFaceData.String, string(capturedDescriptorJSON))

	log.Printf("🔍 Face comparison for %s: %.2f%% similarity", employeeID, similarity)

	// ✅ THRESHOLD: Require at least 45% similarity (lowered from 50%)
	if similarity < 45.0 {
		log.Printf("❌ Face mismatch: %.2f%% < 45%%", similarity)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": fmt.Sprintf("❌ FACE VERIFICATION FAILED\n\nSimilarity: %.1f%% (Required: 45%%)\n\nPlease ensure:\n• Good lighting conditions\n• Face clearly visible inside oval guide\n• Looking directly at camera\n• Remove glasses or face coverings\n• Hold position steady\n\nTry again with better conditions.", similarity),
		})
		return
	}

	log.Printf("✅ Face verified successfully: %.2f%% similarity", similarity)

	// ✅ GET TODAY'S DATE
	today := time.Now().Format("2006-01-02")

	// ✅ HANDLE CHECK-IN
	if req.Action == "check_in" {
		// ✅ VALIDATION: Check if already checked in today
		var alreadyCheckedIn bool
		err := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM attendance 
				WHERE employee_id = ? AND date = ? AND check_in IS NOT NULL
			)
		`, employeeID, today).Scan(&alreadyCheckedIn)

		if err != nil {
			log.Printf("❌ Error checking existing attendance: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database error",
			})
			return
		}

		if alreadyCheckedIn {
			log.Printf("⚠️ Employee %s already checked in today", employeeID)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "⚠️ You have already checked in today! You cannot check in again until tomorrow.",
			})
			return
		}

		// ✅ AUTOMATIC CHECK-IN AFTER FACE VERIFICATION
		_, err = db.Exec(`
			INSERT INTO attendance (employee_id, date, check_in, status)
			VALUES (?, ?, NOW(), 'Present')
			ON DUPLICATE KEY UPDATE check_in = NOW(), status = 'Present'
		`, employeeID, today)

		if err != nil {
			log.Printf("❌ Check-in error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to check in",
			})
			return
		}

		log.Printf("✅ Auto check-in successful: %s (%.2f%% face match)", employeeID, similarity)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("✅ Checked in successfully! (Face match: %.1f%%)\nTime: %s",
				similarity,
				time.Now().Format("3:04 PM")),
		})
		return
	}

	// ✅ HANDLE CHECK-OUT
	if req.Action == "check_out" {
		// ✅ VALIDATION: Check if checked in first
		var hasCheckedIn bool
		err := db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM attendance 
            WHERE employee_id = ? AND date = ? AND check_in IS NOT NULL
        )
    `, employeeID, today).Scan(&hasCheckedIn)

		if err != nil {
			log.Printf("❌ Error checking check-in status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database error",
			})
			return
		}

		if !hasCheckedIn {
			log.Printf("⚠️ Employee %s not checked in yet", employeeID)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "⚠️ You must check in first before you can check out!",
			})
			return
		}

		// ✅ VALIDATION: Check if already checked out today
		var alreadyCheckedOut bool
		err = db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM attendance 
            WHERE employee_id = ? AND date = ? AND check_out IS NOT NULL
        )
    `, employeeID, today).Scan(&alreadyCheckedOut)

		if err != nil {
			log.Printf("❌ Error checking checkout status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Database error",
			})
			return
		}

		if alreadyCheckedOut {
			log.Printf("⚠️ Employee %s already checked out today", employeeID)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "⚠️ You have already checked out today! You cannot check out again.",
			})
			return
		}

		// ✅ VALIDATE: Daily report is required for check-out
		if req.DailyReport == "" || len(req.DailyReport) < 10 {
			log.Printf("❌ Daily report required for employee: %s", employeeID)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "❌ Daily work report is required before checking out (minimum 10 characters)",
			})
			return
		}

		// ✅ PERFORM CHECK-OUT WITH DAILY REPORT
		_, err = db.Exec(`
        UPDATE attendance 
        SET check_out = NOW(), 
            worked_hours = ROUND(TIME_TO_SEC(TIMEDIFF(NOW(), check_in)) / 3600, 2),
            daily_report = ?
        WHERE employee_id = ? AND date = ?
    `, req.DailyReport, employeeID, today)

		if err != nil {
			log.Printf("❌ Check-out error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to check out",
			})
			return
		}

		// ✅ Get worked hours
		var workedHours float64
		db.QueryRow(`
        SELECT COALESCE(worked_hours, 0) 
        FROM attendance 
        WHERE employee_id = ? AND date = ?
    `, employeeID, today).Scan(&workedHours)

		// ✅ Convert to hours and minutes for display
		hours := int(workedHours)
		minutes := int((workedHours - float64(hours)) * 60)

		log.Printf("✅ Check-out successful: %s (%.2f%% face match, %.2f hours worked)",
			employeeID, similarity, workedHours)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("✅ Checked out successfully! (Face match: %.1f%%)\nTime: %s\nHours Worked: %dh %dm\n\n📝 Daily Report Submitted",
				similarity,
				time.Now().Format("3:04 PM"),
				hours,
				minutes),
		})
		return
	}

	// ✅ Invalid action
	log.Printf("❌ Invalid action: %s", req.Action)
	c.JSON(http.StatusBadRequest, gin.H{
		"success": false,
		"message": "Invalid action. Use 'check_in' or 'check_out'",
	})
}

// ✅ IDAGDAG MO ITO SA employee.go

// GetTodayAttendance - Get today's attendance status for logged-in employee
func GetTodayAttendance(c *gin.Context) {
	session := sessions.Default(c)
	employeeID := session.Get("employee_id")

	if employeeID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
		return
	}

	today := time.Now().Format("2006-01-02")

	var record struct {
		CheckIn       sql.NullString
		CheckOut      sql.NullString
		WorkedHours   sql.NullFloat64
		Status        string
		HasCheckedIn  bool
		HasCheckedOut bool
	}

	err := db.QueryRow(`
		SELECT 
			check_in,
			check_out,
			worked_hours,
			status
		FROM attendance
		WHERE employee_id = ? AND date = ?
	`, employeeID, today).Scan(
		&record.CheckIn,
		&record.CheckOut,
		&record.WorkedHours,
		&record.Status,
	)

	if err == sql.ErrNoRows {
		// ✅ No attendance record for today
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"has_checked_in":  false,
				"has_checked_out": false,
				"check_in_time":   nil,
				"check_out_time":  nil,
				"worked_hours":    0,
				"status":          "No Record",
			},
		})
		return
	}

	if err != nil {
		log.Printf("❌ Error fetching today's attendance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error",
		})
		return
	}

	// ✅ Process attendance data
	var checkInTime, checkOutTime *string

	if record.CheckIn.Valid && record.CheckIn.String != "" {
		record.HasCheckedIn = true
		t, _ := time.Parse("2006-01-02 15:04:05", record.CheckIn.String)
		formatted := t.Format("3:04 PM")
		checkInTime = &formatted
	}

	if record.CheckOut.Valid && record.CheckOut.String != "" {
		record.HasCheckedOut = true
		t, _ := time.Parse("2006-01-02 15:04:05", record.CheckOut.String)
		formatted := t.Format("3:04 PM")
		checkOutTime = &formatted
	}

	workedHours := 0.0
	if record.WorkedHours.Valid {
		workedHours = record.WorkedHours.Float64
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"has_checked_in":  record.HasCheckedIn,
			"has_checked_out": record.HasCheckedOut,
			"check_in_time":   checkInTime,
			"check_out_time":  checkOutTime,
			"worked_hours":    workedHours,
			"status":          record.Status,
		},
	})
}

// GetSalaryHistory - Get employee's salary change history
func GetSalaryHistory(c *gin.Context) {
	employeeID := c.Param("id")
	year := c.Query("year") // Optional year filter

	log.Printf("📊 Fetching salary history for employee: %s (year: %s)", employeeID, year)

	query := `
		SELECT 
			id,
			employee_id,
			basic_salary,
			effective_date,
			change_reason,
			changed_by,
			DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
		FROM salary_history
		WHERE employee_id = ?
	`

	args := []interface{}{employeeID}

	// Add year filter if provided
	if year != "" && year != "all" {
		query += " AND YEAR(effective_date) = ?"
		args = append(args, year)
	}

	query += " ORDER BY effective_date DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to load salary history",
		})
		return
	}
	defer rows.Close()

	type SalaryRecord struct {
		ID            int       `json:"id"`
		EmployeeID    string    `json:"employee_id"`
		BasicSalary   float64   `json:"basic_salary"`
		EffectiveDate string    `json:"effective_date"`
		ChangeReason  string    `json:"change_reason"`
		ChangedBy     string    `json:"changed_by"`
		CreatedAt     time.Time `json:"created_at"`
	}

	var history []SalaryRecord
	for rows.Next() {
		var record SalaryRecord
		var createdAtStr string

		err := rows.Scan(
			&record.ID,
			&record.EmployeeID,
			&record.BasicSalary,
			&record.EffectiveDate,
			&record.ChangeReason,
			&record.ChangedBy,
			&createdAtStr,
		)

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		layout := "2006-01-02 15:04:05"
		record.CreatedAt, _ = time.Parse(layout, createdAtStr)

		history = append(history, record)
	}

	log.Printf("✅ Found %d salary records for %s", len(history), employeeID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"history": history,
	})
}

func GetAttendanceHistory(c *gin.Context) {
	employeeID := c.Param("id")

	// Kukunin ang unang araw at huling araw ng current month
	now := time.Now()
	firstDayOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastDayOfMonth := firstDayOfMonth.AddDate(0, 1, -1)

	firstDay := firstDayOfMonth.Format("2006-01-02")
	lastDay := lastDayOfMonth.Format("2006-01-02")

	rows, err := db.Query(`
		SELECT date, check_in, check_out, worked_hours, status, COALESCE(daily_report, '')
		FROM attendance
		WHERE employee_id = ?
		AND date >= ?
		AND date <= ?
		ORDER BY date DESC
	`, employeeID, firstDay, lastDay)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load"})
		return
	}
	defer rows.Close()

	var records []gin.H
	for rows.Next() {
		var date, checkIn, checkOut, dailyReport sql.NullString // ✅ ADD dailyReport HERE
		var workedHours sql.NullFloat64
		var status string

		// ✅ ADD dailyReport to Scan
		err := rows.Scan(&date, &checkIn, &checkOut, &workedHours, &status, &dailyReport)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		records = append(records, gin.H{
			"date":         date.String,
			"check_in":     checkIn.String,
			"check_out":    checkOut.String,
			"worked_hours": workedHours.Float64,
			"status":       status,
			"daily_report": dailyReport.String, // ✅ NOW THIS WORKS
		})
	}

	c.JSON(http.StatusOK, records)
}

// DailyReport struct
type DailyReport struct {
	ID            int       `json:"id"`
	EmployeeID    string    `json:"employee_id"`
	ReportDate    string    `json:"report_date"`
	ReportContent string    `json:"report_content"`
	CreatedAt     time.Time `json:"created_at"`
}

// SubmitDailyReport - Employee submits daily report
func SubmitDailyReport(c *gin.Context) {
	var req struct {
		EmployeeID    string `json:"employee_id"`
		ReportDate    string `json:"report_date"`
		ReportContent string `json:"report_content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	// Validate content length
	if len(req.ReportContent) < 20 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Report content must be at least 20 characters",
		})
		return
	}

	// Check if report already exists for this date
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM daily_reports 
			WHERE employee_id = ? AND report_date = ?
		)
	`, req.EmployeeID, req.ReportDate).Scan(&exists)

	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Database error",
		})
		return
	}

	if exists {
		// Update existing report
		_, err = db.Exec(`
			UPDATE daily_reports 
			SET report_content = ?, created_at = NOW()
			WHERE employee_id = ? AND report_date = ?
		`, req.ReportContent, req.EmployeeID, req.ReportDate)
	} else {
		// Insert new report
		_, err = db.Exec(`
			INSERT INTO daily_reports (employee_id, report_date, report_content, created_at)
			VALUES (?, ?, ?, NOW())
		`, req.EmployeeID, req.ReportDate, req.ReportContent)
	}

	if err != nil {
		log.Printf("❌ Error saving report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save report",
		})
		return
	}

	log.Printf("✅ Daily report saved: %s - %s", req.EmployeeID, req.ReportDate)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Daily report submitted successfully",
	})
}

// GetDailyReportHistory - Get employee's report history
func GetDailyReportHistory(c *gin.Context) {
	employeeID := c.Param("id")

	rows, err := db.Query(`
		SELECT id, employee_id, report_date, report_content, created_at
		FROM daily_reports
		WHERE employee_id = ?
		ORDER BY report_date DESC
		LIMIT 30
	`, employeeID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load"})
		return
	}
	defer rows.Close()

	var reports []DailyReport
	for rows.Next() {
		var report DailyReport
		err := rows.Scan(&report.ID, &report.EmployeeID, &report.ReportDate,
			&report.ReportContent, &report.CreatedAt)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}
		reports = append(reports, report)
	}

	c.JSON(http.StatusOK, reports)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func GetLeaveBalance(c *gin.Context) {
	employeeID := c.Param("id")
	log.Printf("📊 Fetching leave balance for employee: %s", employeeID)

	year := time.Now().Year()

	var sickLeaveUsed, vacationLeaveUsed, emergencyLeaveUsed, maternityLeaveUsed, paternityLeaveUsed int

	// ✅ COUNT BOTH APPROVED AND PENDING (same as RequestLeave)

	// Count Sick Leave used (Approved + Pending)
	db.QueryRow(`
        SELECT COALESCE(SUM(leave_days), 0) 
        FROM leave_requests 
        WHERE employee_id = ? 
        AND leave_type = 'Sick Leave' 
        AND status IN ('Approved', 'Pending')  -- ← FIX
        AND YEAR(leave_from) = ?
    `, employeeID, year).Scan(&sickLeaveUsed)

	// Count Vacation Leave used (Approved + Pending)
	db.QueryRow(`
        SELECT COALESCE(SUM(leave_days), 0) 
        FROM leave_requests 
        WHERE employee_id = ? 
        AND leave_type = 'Vacation Leave' 
        AND status IN ('Approved', 'Pending')  -- ← FIX
        AND YEAR(leave_from) = ?
    `, employeeID, year).Scan(&vacationLeaveUsed)

	// Count Emergency Leave used (Approved + Pending)
	db.QueryRow(`
        SELECT COALESCE(SUM(leave_days), 0) 
        FROM leave_requests 
        WHERE employee_id = ? 
        AND leave_type = 'Emergency Leave' 
        AND status IN ('Approved', 'Pending')  -- ← FIX
        AND YEAR(leave_from) = ?
    `, employeeID, year).Scan(&emergencyLeaveUsed)

	// ✅ Count Maternity Leave used (Approved + Pending)
	db.QueryRow(`
        SELECT COALESCE(SUM(leave_days), 0) 
        FROM leave_requests 
        WHERE employee_id = ? 
        AND leave_type = 'Maternity Leave' 
        AND status IN ('Approved', 'Pending')  -- ← FIX
        AND YEAR(leave_from) = ?
    `, employeeID, year).Scan(&maternityLeaveUsed)

	// ✅ Count Paternity Leave used (Approved + Pending)
	db.QueryRow(`
        SELECT COALESCE(SUM(leave_days), 0) 
        FROM leave_requests 
        WHERE employee_id = ? 
        AND leave_type = 'Paternity Leave' 
        AND status IN ('Approved', 'Pending')  -- ← FIX
        AND YEAR(leave_from) = ?
    `, employeeID, year).Scan(&paternityLeaveUsed)

	// Calculate remaining leave (Total annual allocation - used)
	sickLeaveRemaining := 15 - sickLeaveUsed
	vacationLeaveRemaining := 15 - vacationLeaveUsed
	emergencyLeaveRemaining := 5 - emergencyLeaveUsed
	maternityLeaveRemaining := 105 - maternityLeaveUsed
	paternityLeaveRemaining := 7 - paternityLeaveUsed

	// Make sure they don't go negative
	if sickLeaveRemaining < 0 {
		sickLeaveRemaining = 0
	}
	if vacationLeaveRemaining < 0 {
		vacationLeaveRemaining = 0
	}
	if emergencyLeaveRemaining < 0 {
		emergencyLeaveRemaining = 0
	}
	if maternityLeaveRemaining < 0 {
		maternityLeaveRemaining = 0
	}
	if paternityLeaveRemaining < 0 {
		paternityLeaveRemaining = 0
	}

	response := gin.H{
		"sick_leave":      sickLeaveRemaining,
		"vacation_leave":  vacationLeaveRemaining,
		"emergency_leave": emergencyLeaveRemaining,
		"maternity_leave": maternityLeaveRemaining,
		"paternity_leave": paternityLeaveRemaining,
		"year":            year,
	}

	log.Printf("✅ Leave balance for %s: Sick=%d, Vacation=%d, Emergency=%d, Maternity=%d, Paternity=%d (includes Pending)",
		employeeID, sickLeaveRemaining, vacationLeaveRemaining, emergencyLeaveRemaining,
		maternityLeaveRemaining, paternityLeaveRemaining)

	c.JSON(http.StatusOK, response)
}

// GetHolidaysInMonth returns holidays for a specific month
func GetHolidaysInMonth(c *gin.Context) {
	year := c.Query("year")
	month := c.Query("month")

	if year == "" || month == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Year and month are required",
		})
		return
	}

	log.Printf("📅 Fetching holidays for %s-%s", year, month)

	query := `
        SELECT holiday_date, holiday_name 
        FROM holidays 
        WHERE YEAR(holiday_date) = ? AND MONTH(holiday_date) = ?
        ORDER BY holiday_date
    `

	rows, err := db.Query(query, year, month)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch holidays",
		})
		return
	}
	defer rows.Close()

	type Holiday struct {
		Date string `json:"date"`
		Name string `json:"name"`
	}

	var holidays []Holiday
	for rows.Next() {
		var h Holiday
		if err := rows.Scan(&h.Date, &h.Name); err != nil {
			continue
		}
		holidays = append(holidays, h)
	}

	log.Printf("✅ Found %d holidays in %s-%s", len(holidays), year, month)

	c.JSON(http.StatusOK, gin.H{
		"holidays": holidays,
		"count":    len(holidays),
	})
}

// HRReplyMessage - HR replies to employee/supervisor concerns
func HRReplyMessage(c *gin.Context) {
	var req struct {
		PayrollID int    `json:"payroll_id"`
		Message   string `json:"message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	session := sessions.Default(c)
	username := session.Get("username")
	role := session.Get("role")

	log.Printf("🔍 HR Reply attempt - Username: %v, Role: %v", username, role)

	// ✅ CHECK IF USER IS HR
	if username == nil {
		log.Println("❌ No session found")
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Not authenticated",
		})
		return
	}

	if role == nil || role.(string) != "hr" {
		log.Printf("❌ Access denied - Role is: %v (expected: hr)", role)
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Only HR can reply to concerns",
		})
		return
	}

	// ✅ FIX: Get the payroll's employee_id (not HR's username)
	var payrollEmployeeID string
	err := db.QueryRow("SELECT employee_id FROM payroll WHERE id = ?", req.PayrollID).Scan(&payrollEmployeeID)
	if err != nil {
		log.Printf("❌ Payroll not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Payroll not found",
		})
		return
	}

	// ✅ INSERT REPLY - Use payroll's employee_id, but mark sender as HR
	_, err = db.Exec(`
		INSERT INTO payroll_messages (payroll_id, employee_id, sender_role, message, created_at)
		VALUES (?, ?, 'HR', ?, NOW())
	`, req.PayrollID, payrollEmployeeID, req.Message)

	if err != nil {
		log.Printf("❌ Error sending HR reply: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to send reply",
		})
		return
	}

	log.Printf("✅ HR %s replied to payroll #%d", username, req.PayrollID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reply sent successfully",
	})
}

// GetHRPayrollConcerns - Get all payroll messages FOR HR
func GetHRPayrollConcerns(c *gin.Context) {
	log.Println("📊 Fetching payroll concerns for HR")

	query := `
		SELECT DISTINCT
			p.id as payroll_id,
			p.employee_id,
			CONCAT(e.first_name, ' ', e.last_name) as employee_name,
			p.period_from,
			p.period_to,
			p.net_pay,
			COUNT(pm.id) as message_count,
			DATE_FORMAT(MAX(pm.created_at), '%Y-%m-%d %H:%i:%s') as last_message_time
		FROM payroll_messages pm
		JOIN payroll p ON pm.payroll_id = p.id
		JOIN employees e ON p.employee_id = e.employee_id
		WHERE p.status = 'Approved'
		GROUP BY p.id, p.employee_id, employee_name, p.period_from, p.period_to, p.net_pay
		ORDER BY last_message_time DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("❌ Error fetching HR concerns: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to load concerns",
		})
		return
	}
	defer rows.Close()

	type PayrollConcern struct {
		PayrollID       int       `json:"payroll_id"`
		EmployeeID      string    `json:"employee_id"`
		EmployeeName    string    `json:"employee_name"`
		PeriodFrom      string    `json:"period_from"`
		PeriodTo        string    `json:"period_to"`
		NetPay          float64   `json:"net_pay"`
		MessageCount    int       `json:"message_count"`
		LastMessageTime time.Time `json:"last_message_time"`
	}

	var concerns []PayrollConcern
	for rows.Next() {
		var concern PayrollConcern
		var lastMessageStr string // ✅ FIX: Scan as string first

		err := rows.Scan(&concern.PayrollID, &concern.EmployeeID, &concern.EmployeeName,
			&concern.PeriodFrom, &concern.PeriodTo, &concern.NetPay,
			&concern.MessageCount, &lastMessageStr) // ✅ Scan to string

		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		// ✅ Parse string to time.Time
		layout := "2006-01-02 15:04:05"
		concern.LastMessageTime, _ = time.Parse(layout, lastMessageStr)

		concerns = append(concerns, concern)
	}

	log.Printf("✅ Total HR concerns found: %d", len(concerns))

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"concerns": concerns,
	})
}

// GetPayrollRecordByID - Get single payroll record by ID
func GetPayrollRecordByID(c *gin.Context) {
	recordID := c.Param("id")

	log.Printf("📄 Fetching payroll record ID: %s", recordID)

	query := `
		SELECT 
			id, employee_id, basic_salary, total_working_days, days_worked, prorated_salary,
			COALESCE(regular_holiday_date, ''),
			COALESCE(special_holiday_date, ''),
			COALESCE(regular_holiday_days, 0),
			COALESCE(special_holiday_days, 0),
			COALESCE(regular_holiday_pay, 0),
			COALESCE(special_holiday_pay, 0),
			COALESCE(sick_leave, 0),
			COALESCE(vacation_leave, 0),
			COALESCE(maternity_leave, 0),
			COALESCE(paternity_leave, 0),
			COALESCE(other_leave, 0),
			COALESCE(transportation, 0),
			COALESCE(meal_allowance, 0),
			COALESCE(communication, 0),
			COALESCE(other_allowance, 0),
			COALESCE(sss, 0),
			COALESCE(philhealth, 0),
			COALESCE(pagibig, 0),
			COALESCE(tax, 0),
			COALESCE(cash_advance, 0),
			COALESCE(other_deductions, 0),
			COALESCE(gross_pay, 0),
			COALESCE(total_deductions, 0),
			COALESCE(net_pay, 0),
			period_from,
			period_to,
			IFNULL(status, 'Pending'),
			IFNULL(supervisor_notes, ''),
			DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s') as created_at
		FROM payroll
		WHERE id = ?
	`

	type PayrollRecord struct {
		ID               int     `json:"id"`
		EmployeeID       string  `json:"employee_id"`
		BasicSalary      float64 `json:"basic_salary"`
		TotalWorkingDays int     `json:"total_working_days"`
		DaysWorked       int     `json:"days_worked"`
		ProratedSalary   float64 `json:"prorated_salary"`

		RegularHolidayDate string  `json:"regular_holiday_date"`
		SpecialHolidayDate string  `json:"special_holiday_date"`
		RegularHolidayDays int     `json:"regular_holiday_days"`
		SpecialHolidayDays int     `json:"special_holiday_days"`
		RegularHolidayPay  float64 `json:"regular_holiday_pay"`
		SpecialHolidayPay  float64 `json:"special_holiday_pay"`

		SickLeave      float64 `json:"sick_leave"`
		VacationLeave  float64 `json:"vacation_leave"`
		MaternityLeave float64 `json:"maternity_leave"`
		PaternityLeave float64 `json:"paternity_leave"`
		OtherLeave     float64 `json:"other_leave"`

		Transportation float64 `json:"transportation"`
		MealAllowance  float64 `json:"meal_allowance"`
		Communication  float64 `json:"communication"`
		OtherAllowance float64 `json:"other_allowance"`

		SSS             float64 `json:"sss"`
		PhilHealth      float64 `json:"philhealth"`
		PagIBIG         float64 `json:"pagibig"`
		Tax             float64 `json:"tax"`
		CashAdvance     float64 `json:"cash_advance"`
		OtherDeductions float64 `json:"other_deductions"`

		GrossPay        float64   `json:"gross_pay"`
		TotalDeductions float64   `json:"total_deductions"`
		NetPay          float64   `json:"net_pay"`
		PeriodFrom      string    `json:"period_from"`
		PeriodTo        string    `json:"period_to"`
		Status          string    `json:"status"`
		SupervisorNotes string    `json:"supervisor_notes"`
		CreatedAt       time.Time `json:"created_at"`
	}

	var record PayrollRecord
	var createdAtStr string

	err := db.QueryRow(query, recordID).Scan(
		&record.ID, &record.EmployeeID, &record.BasicSalary, &record.TotalWorkingDays,
		&record.DaysWorked, &record.ProratedSalary,
		&record.RegularHolidayDate, &record.SpecialHolidayDate,
		&record.RegularHolidayDays, &record.SpecialHolidayDays,
		&record.RegularHolidayPay, &record.SpecialHolidayPay,
		&record.SickLeave, &record.VacationLeave, &record.MaternityLeave,
		&record.PaternityLeave, &record.OtherLeave,
		&record.Transportation, &record.MealAllowance, &record.Communication, &record.OtherAllowance,
		&record.SSS, &record.PhilHealth, &record.PagIBIG, &record.Tax,
		&record.CashAdvance, &record.OtherDeductions,
		&record.GrossPay, &record.TotalDeductions, &record.NetPay,
		&record.PeriodFrom, &record.PeriodTo, &record.Status, &record.SupervisorNotes,
		&createdAtStr,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("❌ Payroll record not found: %s", recordID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Payroll record not found"})
			return
		}
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	layout := "2006-01-02 15:04:05"
	record.CreatedAt, _ = time.Parse(layout, createdAtStr)

	log.Printf("✅ Payroll record found: ID=%d, Employee=%s", record.ID, record.EmployeeID)
	c.JSON(http.StatusOK, record)
}

// GetWeeklyTasks - Get attendance/tasks for a specific week
func GetWeeklyTasks(c *gin.Context) {
	employeeID := c.Param("id")
	startDate := c.Query("start")
	endDate := c.Query("end")

	if startDate == "" || endDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Start and end dates required",
		})
		return
	}

	log.Printf("📅 Fetching weekly tasks for %s: %s to %s", employeeID, startDate, endDate)

	query := `
		SELECT 
			date,
			DATE_FORMAT(check_in, '%Y-%m-%d %H:%i:%s') as check_in,
			DATE_FORMAT(check_out, '%Y-%m-%d %H:%i:%s') as check_out,
			COALESCE(daily_report, '') as daily_report,
			COALESCE(worked_hours, 0) as worked_hours,
			status
		FROM attendance
		WHERE employee_id = ?
		AND date >= ?
		AND date <= ?
		ORDER BY date ASC
	`

	rows, err := db.Query(query, employeeID, startDate, endDate)
	if err != nil {
		log.Printf("❌ Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load tasks"})
		return
	}
	defer rows.Close()

	var tasks []gin.H
	for rows.Next() {
		var date, checkIn, checkOut, dailyReport, status sql.NullString
		var workedHours sql.NullFloat64

		err := rows.Scan(&date, &checkIn, &checkOut, &dailyReport, &workedHours, &status)
		if err != nil {
			log.Printf("❌ Scan error: %v", err)
			continue
		}

		tasks = append(tasks, gin.H{
			"date":         date.String,
			"check_in":     checkIn.String,
			"check_out":    checkOut.String,
			"daily_report": dailyReport.String,
			"worked_hours": workedHours.Float64,
			"status":       status.String,
		})
	}

	log.Printf("✅ Found %d task records for week", len(tasks))
	c.JSON(http.StatusOK, tasks)
}

// GetDailyReportsForPeriod - Get daily reports within a date range
func GetDailyReportsForPeriod(c *gin.Context) {
	employeeID := c.Param("id")
	periodFrom := c.Query("period_from")
	periodTo := c.Query("period_to")

	if periodFrom == "" || periodTo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Period dates required",
		})
		return
	}

	log.Printf("📅 Fetching daily reports for %s: %s to %s", employeeID, periodFrom, periodTo)

	// Parse dates
	startDate, err1 := time.Parse("2006-01-02", periodFrom)
	endDate, err2 := time.Parse("2006-01-02", periodTo)

	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid date format",
		})
		return
	}

	// Generate all dates in range
	type DailyReport struct {
		Date        string  `json:"date"`
		DayName     string  `json:"day_name"`
		DailyReport string  `json:"daily_report"`
		CheckIn     string  `json:"check_in"`
		CheckOut    string  `json:"check_out"`
		WorkedHours float64 `json:"worked_hours"`
		HasReport   bool    `json:"has_report"`
	}

	var reports []DailyReport

	// Loop through each day in the period
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		// Check if there's an attendance record for this date
		var reportText sql.NullString
		var checkIn, checkOut sql.NullString
		var workedHours sql.NullFloat64

		err := db.QueryRow(`
			SELECT 
				COALESCE(daily_report, ''),
				DATE_FORMAT(check_in, '%H:%i') as check_in,
				DATE_FORMAT(check_out, '%H:%i') as check_out,
				COALESCE(worked_hours, 0)
			FROM attendance
			WHERE employee_id = ? AND date = ?
		`, employeeID, dateStr).Scan(&reportText, &checkIn, &checkOut, &workedHours)

		report := DailyReport{
			Date:    dateStr,
			DayName: dayName,
		}

		if err == sql.ErrNoRows {
			// No attendance record
			report.DailyReport = "No report"
			report.HasReport = false
		} else if err != nil {
			log.Printf("❌ Query error for %s: %v", dateStr, err)
			report.DailyReport = "Error loading"
			report.HasReport = false
		} else {
			// Has attendance record
			if reportText.Valid && reportText.String != "" {
				report.DailyReport = reportText.String
				report.HasReport = true
			} else {
				report.DailyReport = "No report"
				report.HasReport = false
			}

			if checkIn.Valid {
				report.CheckIn = checkIn.String
			}
			if checkOut.Valid {
				report.CheckOut = checkOut.String
			}
			if workedHours.Valid {
				report.WorkedHours = workedHours.Float64
			}
		}

		reports = append(reports, report)
	}

	log.Printf("✅ Found %d days in period for %s", len(reports), employeeID)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"reports":     reports,
		"period_from": periodFrom,
		"period_to":   periodTo,
	})
}

func RegisterEmployeeRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		// Employee endpoints
		api.GET("/employees/list", ListAllEmployees)
		api.GET("/employee/search/:id", SearchEmployee)
		api.GET("/employee/session", GetCurrentEmployeeSession)

		// Payroll endpoints
		api.POST("/payroll/save", SavePayroll)
		api.GET("/payroll/history/:id", GetPayrollHistory)
		api.POST("/payroll/compute-contributions", ComputeContributions)
		api.GET("/job-grade/salary", GetJobGradeSalary)

		// ✅ PAYROLL MESSAGING
		api.POST("/payroll/message/send", SendPayrollMessage)
		api.GET("/payroll/message/:payroll_id", GetPayrollMessages)
		api.POST("/payroll/message/reply", ManagerReplyMessage)

		// ✅ FIXED: HR gets default /concerns, Manager gets /concerns/manager
		api.GET("/payroll/concerns", GetHRPayrollConcerns)              // ← For HR Dashboard
		api.GET("/payroll/concerns/manager", GetManagerPayrollConcerns) // ← For Manager Dashboard
		api.POST("/payroll/message/hr-reply", HRReplyMessage)

		api.GET("/payroll/record/:id", GetPayrollRecordByID)

		// Supervisor Payroll Review
		api.GET("/payroll/pending", GetPendingPayrolls)
		api.POST("/payroll/approve", ApproveOrRejectPayroll)

		// Manager Routes
		api.GET("/payroll/pending/supervisor", GetPendingSupervisorPayrolls)
		api.POST("/payroll/manager-approve", ManagerApprovePayroll)
		api.POST("/payroll/decision", ManagerApprovePayroll)

		// Leave endpoints
		api.POST("/leave/request", RequestLeave)
		api.GET("/leave/pending/:role", GetPendingLeaves)
		api.POST("/leave/decision", UpdateLeaveDecision)
		api.GET("/leave/history/:id", GetLeaveHistory)
		api.GET("/leave/balance/:id", GetLeaveBalance)

		api.GET("/holidays/month", GetHolidaysInMonth)

		// Attendance endpoints
		api.POST("/attendance/check", CheckAttendance)
		api.GET("/attendance/today", GetTodayAttendance)
		api.GET("/attendance/history/:id", GetAttendanceHistory)
		api.GET("/attendance/week/:id", GetWeeklyTasks)

		api.GET("/salary/history/:id", GetSalaryHistory)

		// Daily Report endpoints
		api.POST("/daily-report/submit", SubmitDailyReport)
		api.GET("/daily-report/history/:id", GetDailyReportHistory)
		api.GET("/daily-report/period/:id", GetDailyReportsForPeriod)
	}

	router.Static("/employee-dashboard", "./static")
	log.Println("✅ Employee routes registered successfully")
}
