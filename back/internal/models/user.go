package models

type User struct {
	Id       int64  `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	Points   int    `json:"points"`
	Password string `json:"-"`
}

type Role string

const (
	ProjectManager    Role = "Project Manager"
	DatabaseEngineer  Role = "Database Engineer"
	FrontendEngineer  Role = "Frontend Engineer"
	BackendEngineer   Role = "Backend Engineer"
	FullstackEngineer Role = "Fullstack Engineer"
	QualityAssuranceE Role = "QA Engineer"
	SoftwareEngineer  Role = "Software Engineer"
	DataScientist     Role = "Data Scientist"
	UIUXDesigner      Role = "UIUX Designer"
)

type PLS string

const (
	GoDeveloper         PLS = "Go Developer"
	RustDeveloper       PLS = "Rust Developer"
	PythonDeveloper     PLS = "Python Developer"
	SwiftDeveloper      PLS = "Swift Developer"
	JavaScriptDeveloper PLS = "JavaScript Developer"
	TypeScriptDeveloper PLS = "TypeScript Developer"
	CPPDeveloper        PLS = "C++ Developer"
	CDeveloper          PLS = "C Developer"
	RubyDeveloper       PLS = "Ruby Developer"
	ZigDeveloper        PLS = "Zig Developer"
	HtmlCssDeveloper    PLS = "Html/Css Developer"
	JavaDeveloper       PLS = "JavaDeveloper"
	KotlinDeveloper     PLS = "Kotlin Developer"
	HaskelDeveloper     PLS = "Haskel Developer"
	CSDeveloper         PLS = "C# Developer"
	X86Developer        PLS = "x86_64 Assembly Developer"
	ARMDeveloper        PLS = "ARM Assembly Developer"
	RISCVDeveloper      PLS = "RISC-V Assembly Developer"
)
