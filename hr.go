package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ========================
// GLOBAL STRUCTS
// ========================

type Employee struct {
	ID          int
	EmployeeID  string
	FirstName   string
	LastName    string
	Name        string
	Email       string
	Contact     string
	Address     string
	CivilStatus string
	Gender      string
	Position    string
	Department  string
	CareerLevel string
	JobGrade    int
	BasicSalary float64
	SalaryType  string
	Status      string
	Role        string
	SSS         string
	PagIBIG     string
	PhilHealth  string
	TIN         string
	DateHired   string
}

// JobGradeSalary represents salary range for each job grade
type JobGradeSalary struct {
	Grade  int     `json:"grade"`
	Salary float64 `json:"salary"`
	Title  string  `json:"title"`
}

// CareerLevel represents the hierarchy of positions
type CareerLevel struct {
	Level      string   `json:"level"`
	MinGrade   int      `json:"min_grade"`
	MaxGrade   int      `json:"max_grade"`
	Categories []string `json:"categories"`
}

// Get Career Levels with corresponding job grades
func getCareerLevels() []CareerLevel {
	return []CareerLevel{
		{
			Level:      "Entry Level",
			MinGrade:   1,
			MaxGrade:   5,
			Categories: []string{"Junior", "Trainee", "Associate"},
		},
		{
			Level:      "Mid Level",
			MinGrade:   6,
			MaxGrade:   12,
			Categories: []string{"Senior", "Lead", "Specialist"},
		},
		{
			Level:      "Senior Level",
			MinGrade:   13,
			MaxGrade:   18,
			Categories: []string{"Principal", "Staff", "Manager"},
		},
		{
			Level:      "Executive Level",
			MinGrade:   19,
			MaxGrade:   25,
			Categories: []string{"Director", "VP", "C-Level"},
		},
	}
}

// API endpoint for career levels
func GetCareerLevels(c *gin.Context) {
	c.JSON(http.StatusOK, getCareerLevels())
}

func getJobGradeSalaries() []JobGradeSalary {
	return []JobGradeSalary{
		// Entry Level (Grades 1-5)
		{1, 18000, "Junior Developer / IT Support I"},
		{2, 22000, "Junior Developer / IT Support II"},
		{3, 26000, "Developer I / System Admin I"},
		{4, 30000, "Developer II / System Admin II"},
		{5, 35000, "Developer III / Network Admin I"},

		// Mid Level (Grades 6-12)
		{6, 40000, "Senior Developer I / DevOps I"},
		{7, 45000, "Senior Developer II / DevOps II"},
		{8, 50000, "Senior Developer III / Security Analyst I"},
		{9, 55000, "Lead Developer I / Solutions Architect I"},
		{10, 60000, "Lead Developer II / Solutions Architect II"},
		{11, 70000, "Lead Developer III / Senior Architect I"},
		{12, 80000, "Principal Developer / Senior Architect II"},

		// Senior Level (Grades 13-18)
		{13, 90000, "Technical Lead I / Engineering Manager I"},
		{14, 100000, "Technical Lead II / Engineering Manager II"},
		{15, 115000, "Senior Technical Lead / Senior Manager I"},
		{16, 130000, "Staff Engineer / Senior Manager II"},
		{17, 145000, "Senior Staff Engineer / Director I"},
		{18, 160000, "Principal Engineer / Director II"},

		// Executive Level (Grades 19-25)
		{19, 180000, "Distinguished Engineer / Senior Director I"},
		{20, 200000, "Fellow / Senior Director II"},
		{21, 225000, "VP Engineering I / Head of IT I"},
		{22, 250000, "VP Engineering II / Head of IT II"},
		{23, 280000, "SVP Engineering / CTO I"},
		{24, 320000, "SVP Technology / CTO II"},
		{25, 380000, "Chief Technology Officer / Chief Information Officer"},
	}
}

// Get salary by job grade
func getSalaryByGrade(grade int) float64 {
	salaries := getJobGradeSalaries()
	for _, s := range salaries {
		if s.Grade == grade {
			return s.Salary
		}
	}
	return 18000 // Default minimum
}

// ========================
// API ENDPOINT FOR JOB GRADES
// ========================

func GetJobGradeSalaries(c *gin.Context) {
	c.JSON(http.StatusOK, getJobGradeSalaries())
}

// ========================
// HR DASHBOARD
// ========================

func HrDashboard(c *gin.Context) {
	rows, err := db.Query("SELECT id, employee_id, first_name, last_name, position, department, status, basic_salary FROM employees WHERE account_status = 'approved'")
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		rows.Scan(&e.ID, &e.EmployeeID, &e.FirstName, &e.LastName, &e.Position, &e.Department, &e.Status, &e.BasicSalary)
		employees = append(employees, e)
	}

	c.HTML(http.StatusOK, "hr_dashboard.html", gin.H{
		"employees": employees,
	})
}

// ========================m
// ADD EMPLOYEE
// ========================

func ShowAddEmployeeForm(c *gin.Context) {
	c.HTML(http.StatusOK, "add_employee.html", nil)
}

func AddEmployee(c *gin.Context) {
	employeeID := fmt.Sprintf("EMP%v", time.Now().UnixNano())

	jobGrade := 0
	fmt.Sscanf(c.PostForm("job_grade"), "%d", &jobGrade)

	// AUTO COMPUTE SALARY BASED ON JOB GRADE
	basicSalary := getSalaryByGrade(jobGrade)

	// ✅ GET FACE DATA FROM FORM
	faceData := c.PostForm("face_data")

	// ✅ VALIDATE DATE HIRED (TODAY ONLY)
	dateHired := c.PostForm("date_hired")
	today := time.Now().Format("2006-01-02")

	if dateHired != today {
		c.String(http.StatusBadRequest, "❌ Date Hired must be TODAY (%s). Cannot hire employees in the past or future.", today)
		return
	}

	// ✅ DETAILED LOGGING
	fmt.Printf("===========================================\n")
	fmt.Printf("📝 EMPLOYEE REGISTRATION DEBUG\n")
	fmt.Printf("Employee ID: %s\n", employeeID)
	fmt.Printf("First Name: %s\n", c.PostForm("first_name"))
	fmt.Printf("Last Name: %s\n", c.PostForm("last_name"))
	fmt.Printf("Date Hired: %s (Today: %s)\n", dateHired, today)
	fmt.Printf("Face Data Received: %v\n", faceData != "")
	fmt.Printf("Face Data Length: %d chars\n", len(faceData))
	if len(faceData) > 0 {
		fmt.Printf("Face Data Preview: %s...\n", faceData[:min(100, len(faceData))])
	}
	fmt.Printf("===========================================\n")

	// ✅ VALIDATE FACE DATA
	if faceData == "" || len(faceData) < 100 {
		fmt.Printf("❌ VALIDATION FAILED: Face data too short or empty\n")
		c.String(http.StatusBadRequest, "❌ Face registration is required! Please capture employee face before submitting.")
		return
	}

	e := Employee{
		EmployeeID:  employeeID,
		FirstName:   c.PostForm("first_name"),
		LastName:    c.PostForm("last_name"),
		Email:       c.PostForm("email"),
		Contact:     c.PostForm("contact"),
		Address:     c.PostForm("address"),
		CivilStatus: c.PostForm("civil_status"),
		Gender:      c.PostForm("gender"),
		CareerLevel: c.PostForm("career_level"),
		JobGrade:    jobGrade,
		BasicSalary: basicSalary,
		Position:    c.PostForm("position"),
		Department:  c.PostForm("department"),
		SalaryType:  c.PostForm("salary_type"),
		Status:      "Active", // ✅ ALWAYS ACTIVE BY DEFAULT
		Role:        c.PostForm("role"),
		SSS:         c.PostForm("sss"),
		PagIBIG:     c.PostForm("pagibig"),
		PhilHealth:  c.PostForm("philhealth"),
		TIN:         c.PostForm("tin"),
		DateHired:   dateHired,
	}

	result, err := db.Exec(`
		INSERT INTO employees
		(employee_id, first_name, last_name, email, contact, address, civil_status, gender,
		 career_level, job_grade, basic_salary, position, department, salary_type, status, role,
		 sss, pagibig, philhealth, tin, date_hired, face_data, account_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		e.EmployeeID, e.FirstName, e.LastName, e.Email, e.Contact, e.Address,
		e.CivilStatus, e.Gender, e.CareerLevel, e.JobGrade, e.BasicSalary,
		e.Position, e.Department, e.SalaryType, e.Status, e.Role,
		e.SSS, e.PagIBIG, e.PhilHealth, e.TIN, e.DateHired,
		faceData, // ✅ FACE DATA SAVED HERE
	)

	if err != nil {
		fmt.Printf("❌ Database INSERT failed: %v\n", err)
		c.String(http.StatusInternalServerError, "Failed to add employee: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("✅ Employee added successfully: %s (Rows affected: %d, Face: %d chars)\n",
		employeeID, rowsAffected, len(faceData))

	// ✅ REDIRECT BASED ON ROLE
	session := sessions.Default(c)
	role := session.Get("role")

	if role != nil && role.(string) == "admin" {
		c.Redirect(http.StatusSeeOther, "/admin/employees?success=Employee+added+successfully")
	} else {
		c.Redirect(http.StatusSeeOther, "/hr/dashboard?success=Employee+added+successfully")
	}
}

// ✅ ADD THIS HELPER FUNCTION (if not exists)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========================
// EDIT EMPLOYEE
// ========================

func ShowEditEmployeeForm(c *gin.Context) {
	employeeID := c.Param("id")

	fmt.Println("===================================")
	fmt.Println("EDIT ROUTE HIT")
	fmt.Println("RAW PARAM ID:", employeeID)
	fmt.Println("===================================")

	var e Employee
	var address, civilStatus, gender sql.NullString // ✅ Use sql.NullString for nullable fields

	err := db.QueryRow(`
		SELECT id, employee_id, first_name, last_name, email, contact, address,
		       civil_status, gender, position, department, career_level, job_grade, 
		       basic_salary, salary_type, status, sss, pagibig, philhealth, tin, date_hired
		FROM employees
		WHERE employee_id = ?
	`, employeeID).Scan(
		&e.ID, &e.EmployeeID, &e.FirstName, &e.LastName, &e.Email, &e.Contact,
		&address, &civilStatus, &gender, &e.Position, &e.Department, // ✅ Scan to sql.NullString
		&e.CareerLevel, &e.JobGrade, &e.BasicSalary, &e.SalaryType, &e.Status,
		&e.SSS, &e.PagIBIG, &e.PhilHealth, &e.TIN, &e.DateHired,
	)

	if err != nil {
		fmt.Println("DB ERROR:", err.Error())
		c.String(http.StatusNotFound, "Employee not found")
		return
	}

	// ✅ Convert sql.NullString to regular string
	if address.Valid {
		e.Address = address.String
	}
	if civilStatus.Valid {
		e.CivilStatus = civilStatus.String
	}
	if gender.Valid {
		e.Gender = gender.String
	}

	jobGrades := []int{}
	for i := 1; i <= 25; i++ {
		jobGrades = append(jobGrades, i)
	}

	session := sessions.Default(c)
	role := session.Get("role")

	c.HTML(http.StatusOK, "edit_employee.html", gin.H{
		"employee":  e,
		"JobGrades": jobGrades,
		"role":      role,
	})
}

func UpdateEmployee(c *gin.Context) {
	employeeID := c.Param("id")

	jobGrade := 0
	fmt.Sscanf(c.PostForm("job_grade"), "%d", &jobGrade)

	// AUTO GET SALARY BASED ON JOB GRADE
	basicSalary := getSalaryByGrade(jobGrade)

	_, err := db.Exec(`
    UPDATE employees SET
    first_name=?, last_name=?, email=?, contact=?, address=?,
    civil_status=?, gender=?, career_level=?, job_grade=?, basic_salary=?,
    position=?, department=?, salary_type=?, status=?, role=?,
    sss=?, pagibig=?, philhealth=?, tin=?, date_hired=?
    WHERE employee_id=?
`,
		c.PostForm("first_name"),
		c.PostForm("last_name"),
		c.PostForm("email"),
		c.PostForm("contact"),
		c.PostForm("address"), // ← ADD THIS
		c.PostForm("civil_status"),
		c.PostForm("gender"), // ← ADD THIS
		c.PostForm("career_level"),
		jobGrade,
		basicSalary,
		c.PostForm("position"),
		c.PostForm("department"),
		c.PostForm("salary_type"),
		c.PostForm("status"),
		c.PostForm("role"),
		c.PostForm("sss"),
		c.PostForm("pagibig"),
		c.PostForm("philhealth"),
		c.PostForm("tin"),
		c.PostForm("date_hired"),
		employeeID,
	)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect based on role
	session := sessions.Default(c)
	role := session.Get("role")

	if role != nil && role.(string) == "admin" {
		c.Redirect(http.StatusFound, "/admin/employees")
	} else {
		c.Redirect(http.StatusFound, "/hr/dashboard")
	}
}

// ========================
// USER MANAGEMENT
// ========================

func ShowUserManagement(c *gin.Context) {
	// ✅ DEFINE STRUCT WITH APPROVED_BY
	type UserDisplay struct {
		ID         int
		Username   string
		Name       string
		Gmail      string
		Role       string
		ApprovedBy string
	}

	// Get all users WITH approved_by info from employees table
	userRows, err := db.Query(`
		SELECT 
			u.id, 
			u.username, 
			u.name, 
			u.gmail, 
			u.role,
			COALESCE(e.approved_by, 'System') as approved_by
		FROM users u
		LEFT JOIN employees e ON u.gmail = e.email
		ORDER BY u.id DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer userRows.Close()

	// ✅ USE NEW STRUCT
	var users []UserDisplay
	for userRows.Next() {
		var u UserDisplay
		userRows.Scan(&u.ID, &u.Username, &u.Name, &u.Gmail, &u.Role, &u.ApprovedBy)
		users = append(users, u)
	}

	pendingRows, err := db.Query(`
		SELECT e.employee_id, e.first_name, e.last_name, e.email, e.position, e.department
		FROM employees e
		WHERE e.account_status = 'pending'
		ORDER BY e.employee_id DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer pendingRows.Close()

	var pendingEmployees []Employee
	for pendingRows.Next() {
		var e Employee
		pendingRows.Scan(&e.EmployeeID, &e.FirstName, &e.LastName, &e.Email, &e.Position, &e.Department)
		pendingEmployees = append(pendingEmployees, e)
	}

	c.HTML(http.StatusOK, "users.html", gin.H{
		"users":            users,
		"pendingEmployees": pendingEmployees,
		"success":          c.Query("success"),
		"error":            c.Query("error"),
	})
}

// Create account for pending employee
func ShowCreateAccountForm(c *gin.Context) {
	employeeID := c.Param("id")

	var e Employee
	err := db.QueryRow(`
		SELECT employee_id, first_name, last_name, email, position, department
		FROM employees
		WHERE employee_id = ?
	`, employeeID).Scan(&e.EmployeeID, &e.FirstName, &e.LastName, &e.Email, &e.Position, &e.Department)

	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Employee+not+found")
		return
	}

	c.HTML(http.StatusOK, "create_account.html", gin.H{
		"employee": e,
	})
}

func CreateAccountFromEmployee(c *gin.Context) {
	employeeID := c.PostForm("employee_id")
	username := c.PostForm("username")
	password := c.PostForm("password")
	role := c.PostForm("role")

	// ✅ GET CURRENT USER (Who is creating the account)
	session := sessions.Default(c)
	approvedBy := session.Get("username")
	if approvedBy == nil {
		approvedBy = "System"
	}

	// Get employee email and name
	var email string
	var name string
	err := db.QueryRow("SELECT email, CONCAT(first_name, ' ', last_name) FROM employees WHERE employee_id = ?", employeeID).Scan(&email, &name)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Employee+not+found")
		return
	}

	// Hash password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	// Insert user
	_, err = db.Exec("INSERT INTO users (username, password, name, gmail, role) VALUES (?, ?, ?, ?, ?)",
		username, string(hashedPassword), name, email, role)

	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+create+account")
		return
	}

	// ✅ UPDATE EMPLOYEE STATUS TO APPROVED
	_, err = db.Exec(`
    UPDATE employees 
    SET account_status = 'approved', role = ?, approved_by = ? 
    WHERE employee_id = ?
`, role, approvedBy, employeeID)
	if err != nil {
		// Rollback: Delete the user if employee update fails
		db.Exec("DELETE FROM users WHERE username = ?", username)
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+approve+employee")
		return
	}

	c.Redirect(http.StatusSeeOther, "/users?success=Account+created+successfully")
}

func AddUserManual(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	name := c.PostForm("name")
	email := c.PostForm("gmail")
	role := c.PostForm("role")

	// ✅ GET CURRENT USER
	session := sessions.Default(c)
	approvedBy := session.Get("username")
	if approvedBy == nil {
		approvedBy = "System"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Password+hashing+failed")
		return
	}

	// ✅ INSERT USER WITH created_by
	_, err = db.Exec(`
		INSERT INTO users (username, password, name, gmail, role, created_by, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, NOW())
	`, username, string(hashedPassword), name, email, role, approvedBy)

	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+add+user")
		return
	}

	c.Redirect(http.StatusSeeOther, "/users?success=User+added+successfully")
}

// Show Edit User Form
func ShowEditUserForm(c *gin.Context) {
	id := c.Param("id")

	var u User
	err := db.QueryRow("SELECT id, username, name, gmail, role FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.Name, &u.Gmail, &u.Role)

	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=User+not+found")
		return
	}

	c.HTML(http.StatusOK, "edit_user.html", gin.H{
		"user": u,
	})
}

// Update User
func UpdateUser(c *gin.Context) {
	id := c.Param("id")
	username := c.PostForm("username")
	name := c.PostForm("name")
	email := c.PostForm("gmail")
	role := c.PostForm("role")
	password := c.PostForm("password")

	// If password is provided, update it
	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/users?error=Password+hashing+failed")
			return
		}

		_, err = db.Exec("UPDATE users SET username=?, password=?, name=?, gmail=?, role=? WHERE id=?",
			username, string(hashedPassword), name, email, role, id)

		if err != nil {
			c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+update+user")
			return
		}
	} else {
		// Update without changing password
		_, err := db.Exec("UPDATE users SET username=?, name=?, gmail=?, role=? WHERE id=?",
			username, name, email, role, id)

		if err != nil {
			c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+update+user")
			return
		}
	}

	c.Redirect(http.StatusSeeOther, "/users?success=User+updated+successfully")
}

func GetAllEmployeesAPI(c *gin.Context) {
	rows, err := db.Query(`
        SELECT id, employee_id, first_name, last_name, email, contact, 
               position, department, status, basic_salary 
        FROM employees
    `)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		err := rows.Scan(&e.ID, &e.EmployeeID, &e.FirstName, &e.LastName,
			&e.Email, &e.Contact, &e.Position, &e.Department,
			&e.Status, &e.BasicSalary)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan error: " + err.Error()})
			return
		}
		employees = append(employees, e)
	}

	c.JSON(http.StatusOK, employees)
}

// Delete User
func DeleteUser(c *gin.Context) {
	id := c.Param("id")

	_, err := db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+delete+user")
		return
	}

	c.Redirect(http.StatusSeeOther, "/users?success=User+deleted+successfully")
}

func getAllEmployees() []Employee {
	rows, err := db.Query(`
        SELECT id, employee_id, first_name, last_name, email, contact, 
               position, department, status, basic_salary, 
               DATE_FORMAT(date_hired, '%b %d, %Y') as date_hired
        FROM employees 
        WHERE account_status = 'approved'
        ORDER BY date_hired DESC
    `)
	if err != nil {
		fmt.Println("Error fetching employees:", err)
		return []Employee{}
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var e Employee
		rows.Scan(&e.ID, &e.EmployeeID, &e.FirstName, &e.LastName,
			&e.Email, &e.Contact, &e.Position, &e.Department,
			&e.Status, &e.BasicSalary, &e.DateHired) // ✅ ADD THIS
		employees = append(employees, e)
	}
	return employees
}

// Add this function in hr.go
func GetApprovedPayrolls(c *gin.Context) {
	rows, err := db.Query(`
		SELECT 
			p.id, p.employee_id, p.basic_salary, p.total_working_days, p.days_worked,
			p.prorated_salary, p.transportation, p.meal_allowance, p.communication,
			p.other_allowance, p.gross_pay, p.sss, p.philhealth, p.pagibig, p.tax,
			p.cash_advance, p.other_deductions, p.total_deductions, p.net_pay,
			p.period_from, p.period_to, p.status, p.payment_status, p.supervisor_notes,
			p.created_at, e.first_name, e.last_name, e.position, e.department
		FROM payroll p
		JOIN employees e ON p.employee_id = e.employee_id
		WHERE p.status = 'Approved'
		ORDER BY p.created_at DESC
	`)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var payrolls []map[string]interface{}

	for rows.Next() {
		var (
			id, totalWorkingDays, daysWorked                                             int
			employeeID, periodFrom, periodTo, status, paymentStatus, firstName, lastName string
			position, department                                                         string
			basicSalary, proratedSalary, transportation, mealAllowance, communication    float64
			otherAllowance, grossPay, sss, philhealth, pagibig, tax                      float64
			cashAdvance, otherDeductions, totalDeductions, netPay                        float64
			supervisorNotes                                                              *string
			createdAt                                                                    string
		)

		err := rows.Scan(
			&id, &employeeID, &basicSalary, &totalWorkingDays, &daysWorked,
			&proratedSalary, &transportation, &mealAllowance, &communication,
			&otherAllowance, &grossPay, &sss, &philhealth, &pagibig, &tax,
			&cashAdvance, &otherDeductions, &totalDeductions, &netPay,
			&periodFrom, &periodTo, &status, &paymentStatus, &supervisorNotes,
			&createdAt, &firstName, &lastName, &position, &department,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan error: " + err.Error()})
			return
		}

		notes := ""
		if supervisorNotes != nil {
			notes = *supervisorNotes
		}

		payroll := map[string]interface{}{
			"id":                 id,
			"employee_id":        employeeID,
			"first_name":         firstName,
			"last_name":          lastName,
			"position":           position,
			"department":         department,
			"basic_salary":       basicSalary,
			"total_working_days": totalWorkingDays,
			"days_worked":        daysWorked,
			"prorated_salary":    proratedSalary,
			"transportation":     transportation,
			"meal_allowance":     mealAllowance,
			"communication":      communication,
			"other_allowance":    otherAllowance,
			"gross_pay":          grossPay,
			"sss":                sss,
			"philhealth":         philhealth,
			"pagibig":            pagibig,
			"tax":                tax,
			"cash_advance":       cashAdvance,
			"other_deductions":   otherDeductions,
			"total_deductions":   totalDeductions,
			"net_pay":            netPay,
			"period_from":        periodFrom,
			"period_to":          periodTo,
			"status":             status,
			"payment_status":     paymentStatus,
			"supervisor_notes":   notes,
			"created_at":         createdAt,
		}

		payrolls = append(payrolls, payroll)
	}

	c.JSON(http.StatusOK, payrolls)
}

// Mark payroll as paid
func MarkPayrollAsPaid(c *gin.Context) {
	var req struct {
		PayrollID int `json:"payroll_id"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	_, err := db.Exec("UPDATE payroll SET payment_status = 'Paid' WHERE id = ?", req.PayrollID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payroll marked as paid"})
}

// ===== PENDING EMPLOYEES PAGE =====
func showPendingEmployees(c *gin.Context) {
	rows, err := db.Query(`
		SELECT employee_id, first_name, last_name, position, department, 
			   email, contact, date_hired, basic_salary, career_level, job_grade
		FROM employees 
		WHERE account_status = 'pending'
		ORDER BY employee_id DESC
	`)
	if err != nil {
		c.String(http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	var pendingEmployees []map[string]interface{}
	for rows.Next() {
		var empID, firstName, lastName, position, dept, email, contact, dateHired, careerLevel string
		var basicSalary float64
		var jobGrade int

		err := rows.Scan(&empID, &firstName, &lastName, &position, &dept, &email, &contact, &dateHired, &basicSalary, &careerLevel, &jobGrade)
		if err != nil {
			continue
		}

		pendingEmployees = append(pendingEmployees, map[string]interface{}{
			"employee_id":  empID,
			"first_name":   firstName,
			"last_name":    lastName,
			"position":     position,
			"department":   dept,
			"email":        email,
			"contact":      contact,
			"date_hired":   dateHired,
			"basic_salary": basicSalary,
			"career_level": careerLevel,
			"job_grade":    jobGrade,
		})
	}

	c.HTML(http.StatusOK, "pending_employees.html", gin.H{
		"pendingEmployees": pendingEmployees,
	})
}

func approveEmployee(c *gin.Context) {
	employeeID := c.Param("id")
	username := c.PostForm("username")
	password := c.PostForm("password")
	role := c.PostForm("role")

	// ✅ GET CURRENT USER (Who is approving)
	session := sessions.Default(c)
	approvedBy := session.Get("username")
	if approvedBy == nil {
		approvedBy = "System"
	}

	if username == "" || password == "" || role == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username, password, and role are required"})
		return
	}

	// Check if username exists
	var existingUser int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&existingUser)
	if existingUser > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	// Get employee details
	var firstName, lastName, email string
	err := db.QueryRow(`
		SELECT first_name, last_name, email 
		FROM employees 
		WHERE employee_id = ? AND account_status = 'pending'
	`, employeeID).Scan(&firstName, &lastName, &email)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found"})
		return
	}

	// Hash password
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), 12)

	// Create user account
	_, err = db.Exec(`
		INSERT INTO users (username, name, password, gmail, role) 
		VALUES (?, ?, ?, ?, ?)
	`, username, firstName+" "+lastName, hashedPassword, email, role)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}

	// ✅ UPDATE EMPLOYEE STATUS TO APPROVED (with approved_by info)
	_, err = db.Exec(`
    UPDATE employees 
    SET account_status = 'approved', role = ?, approved_by = ? 
    WHERE employee_id = ?
`, role, approvedBy, employeeID)

	if err != nil {
		db.Exec("DELETE FROM users WHERE username = ?", username)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Employee approved!"})
}

// ===== REJECT EMPLOYEE =====
func rejectEmployee(c *gin.Context) {
	employeeID := c.Param("id")

	result, err := db.Exec(`
		DELETE FROM employees 
		WHERE employee_id = ? AND account_status = 'pending'
	`, employeeID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Employee not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Employee rejected"})
}

func RegisterHRRoutes(router *gin.Engine) {
	hr := router.Group("/hr")
	{
		hr.GET("/dashboard", HrDashboard)
		hr.GET("/add", ShowAddEmployeeForm)
		hr.POST("/add", AddEmployee)
		hr.GET("/edit/:id", ShowEditEmployeeForm)
		hr.POST("/edit/:id", UpdateEmployee)

		// ✅ ADD THESE 3 LINES:
		hr.GET("/pending-employees", showPendingEmployees)
		hr.POST("/approve-employee/:id", approveEmployee)
		hr.POST("/reject-employee/:id", rejectEmployee)
	}

	router.GET("/api/employees", GetAllEmployeesAPI)
	router.GET("/api/job-grades", GetJobGradeSalaries)
	router.GET("/api/career-levels", GetCareerLevels)
	router.GET("/api/payroll/approved", GetApprovedPayrolls)
	router.POST("/api/payroll/mark-paid", MarkPayrollAsPaid)

}

func RegisterUserRoutes(router *gin.Engine) {
	users := router.Group("/users")
	{
		users.GET("", authRequired(), ShowUserManagement)
		users.GET("/create-account/:id", authRequired(), ShowCreateAccountForm)
		users.POST("/create-account", authRequired(), CreateAccountFromEmployee)
		users.GET("/edit/:id", authRequired(), ShowEditUserForm)
		users.POST("/edit/:id", authRequired(), UpdateUser)
		users.GET("/delete/:id", authRequired(), DeleteUser)
	}
}
