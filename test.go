// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/adminstats.go

package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
)

type userMap map[string]interface{}
type templateMap map[string]interface{}
type workspaceMap map[string]interface{}
type buildMap map[string]interface{}
type projectMap map[string]interface{}

type results struct {
	ID        string
	Name      string
	CreatedBy string
	CreatedAt time.Time
}

type newWorkspace struct {
	ID              string
	Type            string
	ProjectID       string
	OperationStatus string
	CreatedBy       string
	CreatedAt       time.Time
	TfConfig        json.RawMessage `gorm:"column:tfconfig"`
}

// GetProjectStatsAdmin fetches all resource counts for a project
func GetProjectStatsAdmin(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Admin Project Status API Invoked")
	type quotaMap struct {
		Allocated float64 `json:"allocated"`
		Consumed  float64 `json:"consumed"`
	}

	type resourcesQuota struct {
		IDE struct {
			CPUQuota     quotaMap `json:"cpu_quota"`
			MemoryQuota  quotaMap `json:"memory_quota"`
			StorageQuota quotaMap `json:"storage_quota"`
		} `json:"ide"`
		CNV struct {
			CPUQuota     quotaMap `json:"cpu_quota"`
			MemoryQuota  quotaMap `json:"memory_quota"`
			StorageQuota quotaMap `json:"storage_quota"`
		} `json:"cnv"`
	}

	type overview struct {
		Projects   map[string]int `json:"projects"`
		Workspaces map[string]int `json:"workspaces"`
		Templates  map[string]int `json:"templates"`
		Builds     map[string]int `json:"builds"`
		Users      map[string]int `json:"users"`
	}

	type userActivity struct {
		Email string `json:"email"`
		Count int    `json:"count"`
	}

	type loginStats struct {
		TotalLogins int            `json:"total_logins"`
		UserEmails  []string       `json:"user_emails"`
		TopUsers    []userActivity `json:"top_users"`
	}

	type stats struct {
		Overview       overview       `json:"overview"`
		ResourcesQuota resourcesQuota `json:"resources_quota"`
		LoginStats     loginStats     `json:"login_stats"`
	}

	reqID, _ := c.Get("request_id")
	projectID := c.Query("project_id")
	userType := c.GetString("user_type")

	// Check if user is an admin
	if userType != "ADMIN" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}

	var projects []models.Project
	var ws []newWorkspace
	var result []results
	var data []string
	var ideCpuquota, ideMemoryquota, ideStoragequota float64
	var cnvCpuquota, cnvMemoryquota, cnvStoragequota float64

	if projectID != "" {
		// Fetch specific project details
		var project models.Project
		if err := database.DB.Where("name = ? AND is_deleted = ?", projectID, false).Find(&project).Error; err != nil {
			logger.Error("Error fetching project: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error fetching project",
				"request_id": reqID,
			})
			return
		}

		// Check if project exists
		if project.Name == "" {
			logger.Info("No project found with ID: ", projectID)
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "Project not found",
				"request_id": reqID,
			})
			return
		}

		projects = append(projects, project)
		logger.Info("Project found: ", project.Name)
	} else {
		// Fetch all projects
		if err := database.DB.Where("is_deleted = ? AND name != ?", false, "Default").Find(&projects).Error; err != nil {
			logger.Infoln("Projects Fetched: ", err)
			logger.Error("Error fetching projects: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error fetching projects",
				"request_id": reqID,
			})
			return
		}
	}

	for _, project := range projects {
		// Fetch workspaces
		var projectWs []newWorkspace
		if err := database.DB.Table("workspace").Select("*").Where("project_id = ? AND operation_name != ?", project.Name, "DELETION").Scan(&projectWs).Error; err != nil {
			logger.Error("Error fetching workspaces: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error fetching workspaces",
				"request_id": reqID,
			})
			return
		}
		ws = append(ws, projectWs...)
		for _, w := range projectWs {
			var tfConfig map[string]interface{}
			json.Unmarshal(w.TfConfig, &tfConfig)
			if w.Type == "IDE" || w.Type == "Linux_VDI" {
				ideCpuquota += tfConfig["cpu"].(float64)
				ideMemoryquota += tfConfig["memory"].(float64)
				ideStoragequota += tfConfig["drive"].(float64)
			} else if w.Type == "Linux_CNV" || w.Type == "Windows_CNV" || w.Type == "Azure_Linux_VM" || w.Type == "Azure_Windows_VM" {
				cnvCpuquota += tfConfig["cpu"].(float64)
				cnvMemoryquota += tfConfig["memory"].(float64)
				cnvStoragequota += tfConfig["drive"].(float64)
			}
		}

		// Fetch templates
		var projectTemplates []results
		if err := database.DB.Table("template").Select("*").Where("project_id = ? AND operation_name != ?", project.Name, "DELETION").Scan(&projectTemplates).Error; err != nil {
			logger.Error("Error fetching templates: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error fetching templates",
				"request_id": reqID,
			})
			return
		}
		result = append(result, projectTemplates...)

		// Fetch users
		for _, adminID := range project.ProjectAdmins {
			var user models.User
			if err := database.DB.Where("id = ? AND is_deleted=?", adminID, false).Find(&user).Error; err != nil {
				logger.Error("Error fetching admin user: ", err)
				continue
			}
			data = append(data, user.Email)
		}

		for _, memberID := range project.ProjectMembers {
			var user models.User
			if err := database.DB.Where("id = ? AND is_deleted=?", memberID, false).Find(&user).Error; err != nil {
				logger.Error("Error fetching member user: ", err)
				continue
			}
			data = append(data, user.Email)
		}

		// Fetch builds
		var projectBuilds []results
		if err := database.DB.Table("builds").Select("*").Where("project_id = ?", project.Name).Scan(&projectBuilds).Error; err != nil {
			logger.Error("Error fetching builds: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error fetching builds",
				"request_id": reqID,
			})
			return
		}
		result = append(result, projectBuilds...)
	}

	// Calculate the total allocated quotas
	var totalIDECPUQuota, totalIDEMemoryQuota, totalIDEStorageQuota float64
	var totalCNVCPUQuota, totalCNVMemoryQuota, totalCNVStorageQuota float64
	for _, project := range projects {
		totalIDECPUQuota += project.CPUQuota
		totalIDEMemoryQuota += project.MemoryQuota
		totalIDEStorageQuota += project.StorageQuota
		totalCNVCPUQuota += project.VDICPUQuota
		totalCNVMemoryQuota += project.VDIMemoryQuota
		totalCNVStorageQuota += project.VDIStorageQuota
	}

	// --- LOGIN STATS LOGIC ---
	var totalLoginCount int64
	var distinctEmails []string
	var topUsers []userActivity

	// List of admins/system accounts to exclude from the report
	excludeUsers := []string{
		"amartya.anshuman@vodafone.com",
		"bhagyashri.bahirat1@vodafone.com",
		"utkarsh.johari@vodafone.com",
		"prathmesh.kadam@vodafone.com",
		"priyanka.shinde4@vodafone.com",
		"mohd.khan@vodafone.com",
		"shibani.puhan@vodafone.com",
		"akansh.jain@vodafone.com",
		"rutika.parkhe@vodafone.com",
		"vyshak.b@vodafone.com",
		"lalitkumar.patil@vodafone.com",
	}

	// Date Range (One month: today and 30 days ago)

	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, -1, 0)
	startDateStr := startDate.Format("2006-01-02 15:04:05.000000-07")
	endDateStr := endDate.Format("2006-01-02 15:04:05.000000-07")

	// Base query for reuse
	loginQuery := database.DB.Table("audit_logs").
		Where("action = ?", "LOGIN").
		Where("comments = ?", "Logged in to Azure Session").
		Not(map[string]interface{}{"action_performed_by": excludeUsers}).
		Where("action_done_at BETWEEN ? AND ?", startDateStr, endDateStr).
		Limit(10000)

	// 1. Get total login count
	fmt.Println(loginQuery.Statement.SQL.String())
	if err := loginQuery.Count(&totalLoginCount).Error; err != nil {
		logger.Error("Error fetching login count: ", err)
	}

	// 2. Get unique email list (action_performed_by)
	if err := loginQuery.Distinct("action_performed_by").
		Pluck("action_performed_by", &distinctEmails).Error; err != nil {
		logger.Error("Error fetching user emails: ", err)
	}

	// 3. Get Top 5 Users by Login Frequency
	if err := database.DB.Table("audit_logs").
		Select("action_performed_by as email, count(*) as count").
		Where("action = ?", "LOGIN").
		Where("comments = ?", "Logged in to Azure Session").
		Not(map[string]interface{}{"action_performed_by": excludeUsers}).
		Where("action_done_at BETWEEN ? AND ?", startDateStr, endDateStr).
		Group("action_performed_by").
		Order("count DESC").
		Limit(5).
		Scan(&topUsers).Error; err != nil {
		logger.Error("Error fetching top users: ", err)
	}

	// --- DOWNLOAD LOGIN DATA AS CSV ---[GET /enpoint?download_logins=true]
	// if c.Query("download_logins") == "true" {
	// 	c.Writer.Header().Set("Content-Type", "text/csv")
	// 	c.Writer.Header().Set("Content-Disposition", "attachment;filename=login_report.csv")

	// 	writer := csv.NewWriter(c.Writer)
	// 	defer writer.Flush()

	// 	// Write Summary Info
	// 	writer.Write([]string{"Report Name", "User Login Audit (Last 30 Days)"})
	// 	writer.Write([]string{"Report Generated At", time.Now().Format("2006-01-02 15:04:05")})
	// 	writer.Write([]string{"Total Login Frequency", strconv.Itoa(int(totalLoginCount))})
	// 	writer.Write([]string{""})

	// 	// Write Top 5 Users Section
	// 	writer.Write([]string{"TOP 5 USERS (BY ACTIVITY)"})
	// 	writer.Write([]string{"User Email", "Login Count"})
	// 	for _, u := range topUsers {
	// 		writer.Write([]string{u.Email, strconv.Itoa(u.Count)})
	// 	}
	// 	writer.Write([]string{""})

	// 	// Write Email List
	// 	writer.Write([]string{"ALL UNIQUE USER EMAILS"})
	// 	for _, email := range distinctEmails {
	// 		writer.Write([]string{email})
	// 	}
	// 	return
	// }

	stat := stats{
		Overview: overview{
			Projects: map[string]int{
				"count": len(projects),
				"limit": 5,
			},
			Workspaces: map[string]int{
				"count": len(ws),
				"limit": -1,
			},
			Templates: map[string]int{
				"count": len(result),
				"limit": -1,
			},
			Builds: map[string]int{
				"count": len(result),
				"limit": 5,
			},
			Users: map[string]int{
				"count": len(data),
				"limit": 10,
			},
		},
		ResourcesQuota: resourcesQuota{
			IDE: struct {
				CPUQuota     quotaMap `json:"cpu_quota"`
				MemoryQuota  quotaMap `json:"memory_quota"`
				StorageQuota quotaMap `json:"storage_quota"`
			}{
				CPUQuota: quotaMap{
					Allocated: totalIDECPUQuota,
					Consumed:  ideCpuquota,
				},
				MemoryQuota: quotaMap{
					Allocated: totalIDEMemoryQuota,
					Consumed:  ideMemoryquota,
				},
				StorageQuota: quotaMap{
					Allocated: totalIDEStorageQuota,
					Consumed:  ideStoragequota,
				},
			},
			CNV: struct {
				CPUQuota     quotaMap `json:"cpu_quota"`
				MemoryQuota  quotaMap `json:"memory_quota"`
				StorageQuota quotaMap `json:"storage_quota"`
			}{
				CPUQuota: quotaMap{
					Allocated: totalCNVCPUQuota,
					Consumed:  cnvCpuquota,
				},
				MemoryQuota: quotaMap{
					Allocated: totalCNVMemoryQuota,
					Consumed:  cnvMemoryquota,
				},
				StorageQuota: quotaMap{
					Allocated: totalCNVStorageQuota,
					Consumed:  cnvStoragequota,
				},
			},
		},
		LoginStats: loginStats{
			TotalLogins: int(totalLoginCount),
			UserEmails:  distinctEmails,
			TopUsers:    topUsers,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project and User Stats Fetched Successfully.",
		"body":       stat,
		"request_id": reqID,
	})
}


// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/approval.go

// approval contrller

package controllers

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	billingModel "github.com/amartyaa/repas/billing_microservice/billing/models"
	auditlogs "github.com/amartyaa/repas/common/audit_logs"
	commondb "github.com/amartyaa/repas/common/database"
	gitclient "github.com/amartyaa/repas/common/git_client"
	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/middleware"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// Resource is the request body for resorce data
type Resource struct {
	WorkspaceName string `json:"name"`
	Duration      int    `json:"duration"`
	WorkspaceType string `json:"workspace_type"`
	ProjectID     string `json:"project_id"`
}

// WorkspaceResult is the body for storing workspace fetched data
type WorkspaceResult struct {
	Type      string
	ProjectID string
}

// DocumentationApprovalImage represents one documentation image stored inside approval.resource.
type DocumentationApprovalImage struct {
	OriginalFilename string `json:"original_filename"`
	MimeType         string `json:"mime_type"`
	Placeholder      string `json:"placeholder"`
	ImageData        string `json:"image_data"`
}

// AnnouncementApprovalResource represents the resource stored inside the approval request for announcement operations.
type AnnouncementApprovalResource struct {
	AnnouncementID      string          `json:"announcement_id"`
	Action              string          `json:"action"` // CREATE, UPDATE, DELETE
	Heading             string          `json:"heading,omitempty"`
	Subheading          string          `json:"subheading,omitempty"`
	AnnouncementType    string          `json:"announcement_type,omitempty"`
	StartTime           time.Time       `json:"start_time,omitempty"`
	EndTime             time.Time       `json:"end_time,omitempty"`
	OldHeading          string          `json:"old_heading,omitempty"`
	OldSubheading       string          `json:"old_subheading,omitempty"`
	OldAnnouncementType string          `json:"old_announcement_type,omitempty"`
	OldStartTime        time.Time       `json:"old_start_time,omitempty"`
	OldEndTime          time.Time       `json:"old_end_time,omitempty"`
	Payload             json.RawMessage `json:"payload,omitempty"`
}

// DocumentationApprovalResource represents pending create/update/delete section changes.
type DocumentationApprovalResource struct {
	SectionID       int64                        `json:"section_id,omitempty"`
	Action          string                       `json:"action"` // CREATE, UPDATE, DELETE
	Name            string                       `json:"name,omitempty"`
	Content         string                       `json:"content,omitempty"`
	OldContent      string                       `json:"old_content,omitempty"`
	ParentSectionID *int64                       `json:"parent_section_id,omitempty"`
	Images          []DocumentationApprovalImage `json:"images,omitempty"`
}

// ProjectOwnershipApprovalResource represents pending project ownership transfer request.
type ProjectOwnershipApprovalResource struct {
	ProjectID    string     `json:"project_id"`
	TransferType string     `json:"transfer_type"`
	Beneficiary  string     `json:"beneficiary"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
}

// CreateApproval function creates an approval depending on the type of input received
func CreateApproval(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	reqID, _ := c.Get("request_id")
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	userType := c.GetString("user_type")
	ip := c.ClientIP()
	var approval models.Approval
	var title string
	var description string

	err := c.ShouldBind(&approval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	if userType == "ADMIN" && approval.ResourceType != "ANNOUNCEMENT" && approval.ResourceType != "DOCUMENTATION" {
		c.JSON(401, gin.H{
			"error":      "Product Administrators are restricted from creating projects or requesting workspace admin access.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	if approval.ResourceType == "ANNOUNCEMENT" && userType != "ADMIN" {
		c.JSON(401, gin.H{
			"error":      "Only Product Administrators can create announcement approval requests.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	if approval.ResourceType == "DOCUMENTATION" && userType != "ADMIN" {
		c.JSON(401, gin.H{
			"error":      "Only Product Administrators can create documentation approval requests.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	approval.ID = uuid.New()
	if userEmail != approval.RequestFrom {
		c.JSON(404, gin.H{
			"error":      "LoggedIn User Does Not exists. Kindly Login With Correct Credentials",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	logger.Info("Approval :", approval)

	// Combine valid request types from both original functions
	validRequestTypes := []string{
		models.RequestTypeRegistry.ProjectLead,
		models.RequestTypeRegistry.ProjectSubscription,
		models.RequestTypeRegistry.ProjectUpdation,
		models.RequestTypeRegistry.ProjectOwnership,
		models.RequestTypeRegistry.WorkspaceAdmin,
		models.RequestTypeRegistry.WorkspaceDeletion,
		models.RequestTypeRegistry.AnnouncementCreate,
		models.RequestTypeRegistry.AnnouncementUpdate,
		models.RequestTypeRegistry.AnnouncementDelete,
		models.RequestTypeRegistry.DocumentationCreate,
		models.RequestTypeRegistry.DocumentationUpdate,
		models.RequestTypeRegistry.DocumentationDelete,
	}
	result := isAvailable(validRequestTypes, approval.RequestType)
	logger.Info("Is RequestType Valid :", result)
	if result == false {
		c.JSON(400, gin.H{
			"error":      "Provided Request Type Is Not Valid!!!.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	var resourceJSON []byte
	var resourceWorkspace Resource
	var resourceProject models.Project
	var documentationResource DocumentationApprovalResource
	//switch statement
	switch approval.ResourceType {
	case "PROJECT":
		logger.Info("Resource json :", reflect.TypeOf(approval.Resource))
		logger.Info("Resource json :", string(approval.Resource))

		if approval.RequestType == models.RequestTypeRegistry.ProjectOwnership {
			var ownershipResource ProjectOwnershipApprovalResource

			err := json.Unmarshal([]byte(string(approval.Resource)), &ownershipResource)
			if err != nil {
				logger.Errorln(err)
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Invalid project ownership transfer resource payload.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			ownershipResource.ProjectID = strings.TrimSpace(ownershipResource.ProjectID)
			ownershipResource.TransferType = strings.TrimSpace(strings.ToLower(ownershipResource.TransferType))
			ownershipResource.Beneficiary = strings.TrimSpace(strings.ToLower(ownershipResource.Beneficiary))

			if ownershipResource.ProjectID == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "project_id is required for project ownership transfer.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if _, err := uuid.Parse(ownershipResource.ProjectID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Invalid Project ID, Project ID should be UUID",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if ownershipResource.TransferType == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "transfer_type is required.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if ownershipResource.TransferType != "permanent" && ownershipResource.TransferType != "temporary" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Unsupported transfer_type. Supported values: permanent, temporary.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if ownershipResource.TransferType == "temporary" {
				if err := validateTemporaryDelegationWindow(ownershipResource.StartTime, ownershipResource.EndTime); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":      err.Error(),
						"success":    false,
						"request_id": reqID,
					})
					return
				}
			} else if ownershipResource.StartTime != nil || ownershipResource.EndTime != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "start_time and end_time are allowed only for temporary delegation.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if ownershipResource.Beneficiary == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "beneficiary is required.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if !approval.IsExistingResource {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Project ownership transfer is allowed only for existing projects.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			var existingProject models.Project
			res := database.DB.
				Where("id = ? AND is_deleted = ?", ownershipResource.ProjectID, false).
				First(&existingProject)

			if res.Error != nil || res.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Project Not Found",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			currentOwnerEmail := strings.TrimSpace(strings.ToLower(existingProject.AdminEmail))
			requesterEmail := strings.TrimSpace(strings.ToLower(userEmail))
			beneficiaryEmail := strings.TrimSpace(strings.ToLower(ownershipResource.Beneficiary))

			// A temporary delegate has project-admin authority only for the active window.
			if isTemporaryProjectAdmin(existingProject.ID, requesterEmail, time.Now()) {
				c.JSON(http.StatusForbidden, gin.H{
					"error":      "A temporary project admin cannot create permanent or temporary ownership transfer requests.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			isCurrentProjectOwner := strings.EqualFold(
				currentOwnerEmail,
				requesterEmail,
			)

			isSelfOwnershipRequest := strings.EqualFold(
				requesterEmail,
				beneficiaryEmail,
			)

			isBeneficiaryDevOpsLead := isBeneficiaryProjectAdmin(
				existingProject,
				beneficiaryEmail,
			)

			// Current project owner selects one of the project's DevOps Leads as the new permanent project owner.
			// One of the project's DevOps Leads requests permanent project ownership for themselves.
			// A DevOps Lead cannot request ownership for another DevOps Lead.
			if !isCurrentProjectOwner && !isSelfOwnershipRequest {
				c.JSON(http.StatusForbidden, gin.H{
					"error":      "Only the current project owner can nominate another DevOps Lead. A DevOps Lead can request ownership only for themselves.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			// Both owner delegation and DevOps Lead self-request require the beneficiary to be an existing project admin/DevOps Lead.
			if !isBeneficiaryDevOpsLead {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Beneficiary must be one of the project admins.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			// The current project owner cannot be the beneficiary
			if strings.EqualFold(currentOwnerEmail, beneficiaryEmail) {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Project ownership cannot be transferred to the current owner.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if ownershipResource.TransferType == "temporary" && hasDelegationOverlap(existingProject.ID, *ownershipResource.StartTime, *ownershipResource.EndTime) {
				c.JSON(http.StatusConflict, gin.H{
					"error":      "A temporary project-admin delegation overlaps an existing delegation for this project.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			var pendingApprovals []models.Approval
			_ = database.DB.Where(
				"resource_type = ? AND status = ? AND request_type = ?",
				"PROJECT",
				models.ApprovalStatus.PENDING,
				models.RequestTypeRegistry.ProjectOwnership,
			).Find(&pendingApprovals)

			for i := range pendingApprovals {
				var pendingOwnership ProjectOwnershipApprovalResource
				err = json.Unmarshal([]byte(string(pendingApprovals[i].Resource)), &pendingOwnership)
				if err != nil {
					logger.Info(err)
					continue
				}

				if pendingOwnership.ProjectID == ownershipResource.ProjectID {
					if ownershipResource.TransferType == "permanent" || pendingOwnership.TransferType == "permanent" {
						c.JSON(http.StatusConflict, gin.H{
							"error":      "A pending ownership transfer approval already exists for this project.",
							"success":    false,
							"request_id": reqID,
						})
						return
					}
					if ownershipResource.StartTime != nil && ownershipResource.EndTime != nil && pendingOwnership.StartTime != nil && pendingOwnership.EndTime != nil && ownershipResource.StartTime.Before(*pendingOwnership.EndTime) && ownershipResource.EndTime.After(*pendingOwnership.StartTime) {
						c.JSON(http.StatusConflict, gin.H{
							"error":      "A pending temporary delegation overlaps an existing request for this project.",
							"success":    false,
							"request_id": reqID,
						})
						return
					}
				}
			}

			var users []models.User
			var platformAdminsEmail pq.StringArray

			database.DB.Where("type = ?", "ADMIN").Find(&users)
			for i := 0; i < len(users); i++ {
				if users[i].ID.String() == userID {
					continue
				}

				platformAdminsEmail = append(platformAdminsEmail, users[i].ID.String())
			}

			if len(platformAdminsEmail) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "No platform admin available to approve this ownership transfer request.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			approval.RequestTo = platformAdminsEmail
			title = "Raised A Request For " + approval.RequestType + "!!!"
			if isCurrentProjectOwner {
				description = userEmail + " has raised a request to transfer project ownership for project " + existingProject.Name + " to " + ownershipResource.Beneficiary + "."
			} else {
				description = userEmail + " has raised a request to become the permanent owner of project " + existingProject.Name + "."
			}

			resourceJSON, _ = json.Marshal(ownershipResource)
			resourceProject = existingProject
		} else {
			err := json.Unmarshal([]byte(string(approval.Resource)), &resourceProject)
			if err != nil {
				logger.Errorln(err)
				return
			}
			//Throw an error if the project name is not given.
			if len(resourceProject.Name) == 0 {
				c.JSON(400, gin.H{
					"error":      "Project Name Not Given...",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			// Throw an error if the user is already raised an request for the resource type which is not resolved yet.
			if approval.ResourceType == "PROJECT" && len(resourceProject.Name) > 0 {
				var pendingApprovals []models.Approval
				_ = database.DB.Where("request_from = ? AND status = ?", userID, models.ApprovalStatus.PENDING).Find(&pendingApprovals)
				logger.Info("LoggedIn User :", userEmail)
				for i := range pendingApprovals {
					var pendingApprovalResourceProject models.Project
					err = json.Unmarshal([]byte(string(pendingApprovals[i].Resource)), &pendingApprovalResourceProject)
					if err != nil {
						logger.Info(err)
						return
					}
					if pendingApprovals[i].RequestFrom == userID && pendingApprovals[i].RequestType == approval.RequestType && pendingApprovalResourceProject.Name == resourceProject.Name {
						if pendingApprovals[i].RequestType == models.RequestTypeRegistry.ProjectSubscription {
							c.JSON(409, gin.H{
								"error":      userEmail + " already raised a request to become a member for the requested project named " + resourceProject.Name + ".",
								"success":    false,
								"request_id": reqID,
							})
							return
						}
						if pendingApprovals[i].RequestType == models.RequestTypeRegistry.ProjectLead {
							c.JSON(409, gin.H{
								"error":      userEmail + " already raised a request to become a admin for the requested project named " + resourceProject.Name + ".",
								"success":    false,
								"request_id": reqID,
							})
							return
						}
						if pendingApprovals[i].RequestType == models.RequestTypeRegistry.ProjectUpdation {
							c.JSON(409, gin.H{
								"error":      userEmail + " already raised a request to update the quotas for project named " + resourceProject.Name + ".",
								"success":    false,
								"request_id": reqID,
							})
							return
						}
					}
				}
			}

			if approval.RequestType == "PROJECT_LEAD" || approval.RequestType == "PROJECT_SUBSCRIPTION" {
				if approval.IsExistingResource == true {
					var project models.Project
					result := isProjectExists(resourceProject.Name)
					logger.Info("Is Project Exists :", result)
					if result {
						_ = database.DB.Select("id").Where("name = ?", resourceProject.Name).Find(&project)
						if approval.RequestType == "PROJECT_SUBSCRIPTION" {
							isUserEntryAllowed := checkProjectLimit(project.ID)
							logger.Info("isUserEntryAllowed : ", isUserEntryAllowed)
							if isUserEntryAllowed == false {
								c.JSON(404, gin.H{
									"error":      "The USER Quota for the project has been Exceeded, Please Contact Platform Admin",
									"success":    false,
									"request_id": reqID,
								})
								return
							}
						}
						var userEmails pq.StringArray
						var user models.User
						var project models.Project
						_ = database.DB.Where("name = ?", resourceProject.Name).Find(&project)
						logger.Info("admin emails for the project :", project.ProjectAdmins)
						for i := 0; i < len(project.ProjectAdmins); i++ {
							database.DB.Where("id = ?", project.ProjectAdmins[i]).Find(&user)
							userEmails = append(userEmails, user.ID.String())

							if project.ProjectAdmins[i] == userID {
								c.JSON(404, gin.H{
									"error":      userEmail + " already having the DevOps_Lead access for requested project " + resourceProject.Name + ".",
									"success":    false,
									"request_id": reqID,
								})
								return
							}
							user = models.User{}
						}
						logger.Info("admin emails for the project :", userEmails)
						approval.RequestTo = userEmails
						title = "Raised A Request For " + approval.RequestType + "!!!"
						if approval.RequestType == models.RequestTypeRegistry.ProjectLead {
							description = approval.RequestFrom + " has raised a request to get DevOps_Lead access for the project " + resourceProject.Name + "."
						} else {
							description = approval.RequestFrom + " has raised a request to get DevOps access for the project " + resourceProject.Name + "."
							for i := 0; i < len(project.ProjectMembers); i++ {
								if project.ProjectMembers[i] == userID {
									c.JSON(409, gin.H{
										"error":      userEmail + " already having the DevOps access for requested project " + resourceProject.Name + ".",
										"success":    false,
										"request_id": reqID,
									})
									return
								}
							}
						}
					} else {
						c.JSON(404, gin.H{
							"error":      "The Project Name You Entered Does Not Seem To Be Valid...",
							"success":    false,
							"request_id": reqID,
						})
						return
					}
				}

				if approval.IsExistingResource == false {
					if result = isProjectExists(resourceProject.Name); result == true {
						if isProjectRecentlyDeleted(resourceProject.Name) {
							c.JSON(409, gin.H{
								"error":      "Project with the same name was recently deleted. Please choose a different name.",
								"success":    false,
								"request_id": reqID,
							})
							return
						}
						c.JSON(409, gin.H{
							"success":    false,
							"error":      "Project Already Exists With Given Project Name",
							"request_id": reqID,
						})
						return
					}

					if resourceProject.Name == "" || !isValidRFC1123(resourceProject.Name) {
						c.JSON(http.StatusBadRequest, gin.H{
							"error":      "Name Is required and should be valid RFC1123",
							"request_id": reqID,
						})
						return
					}
					var users []models.User
					var platformAdminsEmail pq.StringArray
					database.DB.Where("type = ?", "ADMIN").Find(&users)
					for i := 0; i < len(users); i++ {
						platformAdminsEmail = append(platformAdminsEmail, users[i].ID.String())
					}
					approval.RequestTo = platformAdminsEmail
					title = "Raised A Request For " + approval.RequestType + "!!!"
					description = approval.RequestFrom + " has raised a request to onboard a new Project with DevOps_Lead access for " + resourceProject.Name + "."
				}
			}
			if approval.RequestType == "PROJECT_UPDATION" {
				// Users with devopsLead access are only authorized to perform this action(update the project).
				permissions := middleware.FetchPermissions(userID)
				devopsLead := permissions["DevOps_Lead"].([]string)
				if result := Contains(devopsLead, resourceProject.Name); !result {
					c.JSON(401, gin.H{
						"success":    false,
						"error":      "You are not authorized to perform PROJECT_UPDATION action",
						"request_id": reqID,
					})
					return
				}

				if !approval.IsExistingResource {
					c.JSON(http.StatusBadRequest, gin.H{
						"success":    false,
						"error":      "Make sure the project you want to update exists before proceeding.",
						"request_id": reqID,
					})
					return
				}

				var users []models.User
				var existingProject models.Project
				var platformAdminsEmail pq.StringArray
				description = approval.RequestFrom + " has raised a request to update the project quota for " + resourceProject.Name + "."

				//Validations on increment of build quota
				_ = database.DB.Where("name = ?", resourceProject.Name).Find(&existingProject)

				if (resourceProject.BuildQuota == existingProject.BuildQuota) && (resourceProject.CPUQuota == existingProject.CPUQuota) && (resourceProject.MemoryQuota == existingProject.MemoryQuota) && (resourceProject.StorageQuota == existingProject.StorageQuota) && (resourceProject.VDIBuildQuota == existingProject.VDIBuildQuota) && (resourceProject.VDICPUQuota == existingProject.VDICPUQuota) && (resourceProject.VDIMemoryQuota == existingProject.VDIMemoryQuota) && (resourceProject.VDIStorageQuota == existingProject.VDIStorageQuota) {
					c.JSON(400, gin.H{
						"success":    false,
						"error":      "Requested quota is same as existing quota",
						"request_id": reqID,
					})
					return
				}
				if resourceProject.CPUQuota > existingProject.CPUQuota {
					description = description + "Current CPU quota for IDE is set to: " + fmt.Sprint(existingProject.CPUQuota) + " requesting to update to " + fmt.Sprint(resourceProject.CPUQuota) + "."
				}
				if resourceProject.MemoryQuota != 0 {
					if (resourceProject.MemoryQuota < models.DefaultQuota.Memory) || (resourceProject.MemoryQuota < existingProject.MemoryQuota) {
						c.JSON(400, gin.H{
							"success":    false,
							"error":      "The memory quota has a lower limit of " + fmt.Sprint(models.DefaultQuota.Memory) + " GB and the new memory quota should be greater than the existing memory Quota.",
							"request_id": reqID,
						})
						return
					}
					if resourceProject.MemoryQuota > existingProject.MemoryQuota {
						description = description + "Current Memory quota for IDE is set to: " + fmt.Sprint(existingProject.MemoryQuota) + " requesting to update to " + fmt.Sprint(resourceProject.MemoryQuota) + "."
					}
				}
				if resourceProject.StorageQuota != 0 {
					if (resourceProject.StorageQuota < models.DefaultQuota.Storage) || (resourceProject.StorageQuota < existingProject.StorageQuota) {
						c.JSON(400, gin.H{
							"success":    false,
							"error":      "The storage quota has a lower limit of " + fmt.Sprint(models.DefaultQuota.Storage) + " GB and new the storage quota should be greater than the existing storage Quota.",
							"request_id": reqID,
						})
						return
					}
					if resourceProject.StorageQuota > existingProject.StorageQuota {
						description = description + "Current Storage quota for IDE is set to: " + fmt.Sprint(existingProject.StorageQuota) + " requesting to update to" + fmt.Sprint(resourceProject.StorageQuota) + "."
					}
					if resourceProject.CPUQuota != 0 {
						//Validations on increment of computational quota
						if (resourceProject.CPUQuota < models.DefaultQuota.CPU) || (resourceProject.CPUQuota < existingProject.CPUQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The CPU quota has a lower limit of " + fmt.Sprint(models.DefaultQuota.CPU) + " cores and the new CPU quota should be greater than the existing CPU Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.CPUQuota > existingProject.CPUQuota {
							description = description + "Current CPU quota for IDE is set to: " + fmt.Sprint(existingProject.CPUQuota) + " requesting to update to " + fmt.Sprint(resourceProject.CPUQuota) + "."
						}
					}
					if resourceProject.MemoryQuota != 0 {
						if (resourceProject.MemoryQuota < models.DefaultQuota.Memory) || (resourceProject.MemoryQuota < existingProject.MemoryQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The memory quota has a lower limit of " + fmt.Sprint(models.DefaultQuota.Memory) + " GB and the new memory quota should be greater than the existing memory Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.MemoryQuota > existingProject.MemoryQuota {
							description = description + "Current Memory quota for IDE is set to: " + fmt.Sprint(existingProject.MemoryQuota) + " requesting to update to " + fmt.Sprint(resourceProject.MemoryQuota) + "."
						}
					}
					if resourceProject.StorageQuota != 0 {
						if (resourceProject.StorageQuota < models.DefaultQuota.Storage) || (resourceProject.StorageQuota < existingProject.StorageQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The storage quota has a lower limit of " + fmt.Sprint(models.DefaultQuota.Storage) + " GB and new the storage quota should be greater than the existing storage Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.StorageQuota > existingProject.StorageQuota {
							description = description + "Current Storage quota for IDE is set to: " + fmt.Sprint(existingProject.StorageQuota) + " requesting to update to" + fmt.Sprint(resourceProject.StorageQuota) + "."
						}
					}
					if resourceProject.VDIBuildQuota != 0 {
						if (resourceProject.VDIBuildQuota < models.VDIDefaultQuota.Build) || (resourceProject.VDIBuildQuota < existingProject.VDIBuildQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The minimum limit to increase the build quota is " + fmt.Sprint(models.VDIDefaultQuota.Build) + " and the new build quota should be greater than the existing build Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.VDIBuildQuota > existingProject.VDIBuildQuota {
							description = description + "Current Build quota for VDI is set to: " + fmt.Sprint(existingProject.VDIBuildQuota) + " requesting to update to " + fmt.Sprint(resourceProject.VDIBuildQuota) + "."
						}
					}
					if resourceProject.VDICPUQuota != 0 {
						//Validations on increment of computational quota
						if (resourceProject.VDICPUQuota < models.VDIDefaultQuota.CPU) || (resourceProject.VDICPUQuota < existingProject.VDICPUQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The CPU quota has a lower limit of " + fmt.Sprint(models.VDIDefaultQuota.CPU) + " cores and the new CPU quota should be greater than the existing CPU Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.VDICPUQuota > existingProject.VDICPUQuota {
							description = description + "Current CPU quota for VDI is set to: " + fmt.Sprint(existingProject.VDICPUQuota) + " requesting to update to " + fmt.Sprint(resourceProject.VDICPUQuota) + "."
						}
					}
					if resourceProject.VDIMemoryQuota != 0 {
						if (resourceProject.VDIMemoryQuota < models.VDIDefaultQuota.Memory) || (resourceProject.VDIMemoryQuota < existingProject.VDIMemoryQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The memory quota has a lower limit of " + fmt.Sprint(models.VDIDefaultQuota.Memory) + " GB and the new memory quota should be greater than the existing memory Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.VDIMemoryQuota > existingProject.VDIMemoryQuota {
							description = description + "Current Memory quota for VDI is set to: " + fmt.Sprint(existingProject.VDIMemoryQuota) + " requesting to update to " + fmt.Sprint(resourceProject.VDIMemoryQuota) + "."
						}
					}
					if resourceProject.VDIStorageQuota != 0 {
						if (resourceProject.VDIStorageQuota < models.VDIDefaultQuota.Storage) || (resourceProject.VDIStorageQuota < existingProject.VDIStorageQuota) {
							c.JSON(400, gin.H{
								"success":    false,
								"error":      "The storage quota has a lower limit of " + fmt.Sprint(models.VDIDefaultQuota.Storage) + " GB and new the storage quota should be greater than the existing storage Quota.",
								"request_id": reqID,
							})
							return
						}
						if resourceProject.VDIStorageQuota > existingProject.VDIStorageQuota {
							description = description + "Current Storage quota for VDI is set to: " + fmt.Sprint(existingProject.VDIStorageQuota) + " requesting to update to " + fmt.Sprint(resourceProject.VDIStorageQuota) + "."
						}
					}
				}

				database.DB.Where("type = ?", "ADMIN").Find(&users)
				for i := range users {
					platformAdminsEmail = append(platformAdminsEmail, users[i].ID.String())
				}

				approval.RequestTo = platformAdminsEmail
				title = "Raised A Request For " + approval.RequestType + "!!!"
			}
			logger.Infof("description :", description)
			resourceJSON, _ = json.Marshal(resourceProject)
		}

	case "WORKSPACE":
		logger.Info("Resource json :", reflect.TypeOf(approval.Resource))
		logger.Info("Resource json :", string(approval.Resource))
		err := json.Unmarshal([]byte(string(approval.Resource)), &resourceWorkspace)
		if err != nil {
			logger.Errorln(err)
			return
		}
		//Throw an error if the workspace name is not given.
		if len(resourceWorkspace.WorkspaceName) == 0 {
			c.JSON(400, gin.H{
				"error":      "Workspace Name not given...........",
				"success":    false,
				"request_id": reqID,
			})
			return
		}
		var WorkspaceResult WorkspaceResult
		res := database.DB.Raw("SELECT type ,project_id FROM workspace WHERE id = $1", resourceWorkspace.WorkspaceName).Scan(&WorkspaceResult)
		if res.RowsAffected == 0 {
			c.JSON(500, gin.H{
				"error":      "Invalid Workspace name..........",
				"success":    false,
				"request_id": reqID,
			})
			return
		}
		resourceWorkspace.WorkspaceType = WorkspaceResult.Type
		resourceWorkspace.ProjectID = WorkspaceResult.ProjectID
		// Throw an error if the user is already raised an request for the resource type which is not resolved yet.
		if approval.ResourceType == "WORKSPACE" && len(resourceWorkspace.WorkspaceName) != 0 {
			var pendingApprovals []models.Approval
			_ = database.DB.Where("request_from = ? AND status = ?", userID, models.ApprovalStatus.PENDING).Find(&pendingApprovals)
			logger.Info("LoggedIn User :", userEmail)
			for i := range pendingApprovals {
				var pendingApprovalResourceWorkspace Resource
				err = json.Unmarshal([]byte(string(pendingApprovals[i].Resource)), &pendingApprovalResourceWorkspace)
				if err != nil {
					logger.Info(err)
					return
				}
				if pendingApprovals[i].RequestFrom == userID && pendingApprovals[i].RequestType == approval.RequestType && pendingApprovalResourceWorkspace.WorkspaceName == resourceWorkspace.WorkspaceName {
					if pendingApprovals[i].RequestType == models.RequestTypeRegistry.WorkspaceAdmin {
						c.JSON(409, gin.H{
							"message":    userEmail + " already raised a request to get admin access for workspace :" + resourceWorkspace.WorkspaceName + ".",
							"success":    false,
							"request_id": reqID,
						})
						return
					}
					if pendingApprovals[i].RequestType == models.RequestTypeRegistry.WorkspaceDeletion {
						c.JSON(409, gin.H{
							"message":    userEmail + " already raised a request to delete workspace :" + resourceWorkspace.WorkspaceName + ".",
							"success":    false,
							"request_id": reqID,
						})
						return
					}
				}
			}
		}
		if approval.RequestType == "WORKSPACE_ADMIN" {
			if approval.IsExistingResource == true {
				var users []models.User
				var platformAdminsEmail pq.StringArray
				database.DB.Where("type = ?", "ADMIN").Find(&users)
				for i := 0; i < len(users); i++ {
					platformAdminsEmail = append(platformAdminsEmail, users[i].ID.String())
				}
				approval.RequestTo = platformAdminsEmail
				title = "Raised A Request For " + approval.RequestType + "!!!"
				description = userEmail + " has raised a request to get workspace admin access for workspace " + resourceWorkspace.WorkspaceName + "."
			}
		}
		if approval.RequestType == "WORKSPACE_DELETION" {
			userID := c.GetString("user_id")

			permissions := middleware.FetchPermissions(userID)
			logger.Infoln("Permissions :", permissions)
			devopsLead := permissions["DevOps_Lead"].([]string)
			devops := permissions["DevOps"].([]string)

			if approval.IsExistingResource == true {
				var users []models.User
				var existingProject models.Project
				var platformAdminsEmail pq.StringArray

				if isAvailable(devopsLead, resourceWorkspace.ProjectID) {
					database.DB.Where("type = ?", "ADMIN").Find(&users)
					for i := 0; i < len(users); i++ {
						platformAdminsEmail = append(platformAdminsEmail, users[i].ID.String())
					}
					approval.RequestTo = platformAdminsEmail
				}

				if isAvailable(devops, resourceWorkspace.ProjectID) {
					_ = database.DB.Where("name = ?", resourceWorkspace.ProjectID).Find(&existingProject)
					approval.RequestTo = existingProject.ProjectAdmins
				}

				title = "Raised A Request For " + approval.RequestType + "!!!"
				description = userEmail + " has raised a request to delete workspace " + resourceWorkspace.WorkspaceName + "."
			}
		}
		resourceJSON, _ = json.Marshal(resourceWorkspace)

	case "ANNOUNCEMENT":
		logger.Info("Announcement resource json:", string(approval.Resource))

		var announcementResource AnnouncementApprovalResource

		err := json.Unmarshal([]byte(string(approval.Resource)), &announcementResource)
		if err != nil {
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid announcement resource payload.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		if announcementResource.Action == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "announcement action is required.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		validAnnouncementRequestTypes := []string{
			models.RequestTypeRegistry.AnnouncementCreate,
			models.RequestTypeRegistry.AnnouncementUpdate,
			models.RequestTypeRegistry.AnnouncementDelete,
		}

		if !isAvailable(validAnnouncementRequestTypes, approval.RequestType) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid announcement request type.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		validActionForRequest := map[string]string{
			models.RequestTypeRegistry.AnnouncementCreate: "CREATE",
			models.RequestTypeRegistry.AnnouncementUpdate: "UPDATE",
			models.RequestTypeRegistry.AnnouncementDelete: "DELETE",
		}

		expectedAction := validActionForRequest[approval.RequestType]

		if announcementResource.Action != expectedAction {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "announcement action does not match request type.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		if approval.RequestType == models.RequestTypeRegistry.AnnouncementCreate {
			if announcementResource.AnnouncementID == "" {
				announcementResource.AnnouncementID = uuid.New().String()
			}

			if err := validateAnnouncementApprovalPayload(announcementResource); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      err.Error(),
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		if approval.RequestType == models.RequestTypeRegistry.AnnouncementUpdate {
			if announcementResource.AnnouncementID == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "announcement_id is required for update.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if err := validateAnnouncementApprovalPayload(announcementResource); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      err.Error(),
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		if approval.RequestType == models.RequestTypeRegistry.AnnouncementDelete {
			if announcementResource.AnnouncementID == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "announcement_id is required for delete.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		// Resource snapshot: for UPDATE/DELETE, pull the current DB values.
		if approval.RequestType == models.RequestTypeRegistry.AnnouncementUpdate || approval.RequestType == models.RequestTypeRegistry.AnnouncementDelete {
			type announcementSnapshot struct {
				Heading          string
				Subheading       string
				AnnouncementType string
				StartTime        time.Time
				EndTime          time.Time
			}

			var existingAnnouncement announcementSnapshot
			annDBRes := database.DB.Raw(
				"SELECT heading, subheading, announcement_type, start_time, end_time FROM announcements WHERE id = $1 AND deleted_at IS NULL",
				announcementResource.AnnouncementID,
			).Scan(&existingAnnouncement)

			if annDBRes.Error != nil {
				logger.Errorln(annDBRes.Error)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      "Error while fetching existing announcement.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if annDBRes.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Announcement not found.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if approval.RequestType == models.RequestTypeRegistry.AnnouncementUpdate {
				announcementResource.OldHeading = existingAnnouncement.Heading
				announcementResource.OldSubheading = existingAnnouncement.Subheading
				announcementResource.OldAnnouncementType = existingAnnouncement.AnnouncementType
				announcementResource.OldStartTime = existingAnnouncement.StartTime
				announcementResource.OldEndTime = existingAnnouncement.EndTime
			}

			if approval.RequestType == models.RequestTypeRegistry.AnnouncementDelete {
				announcementResource.Heading = existingAnnouncement.Heading
				announcementResource.Subheading = existingAnnouncement.Subheading
				announcementResource.AnnouncementType = existingAnnouncement.AnnouncementType
				announcementResource.StartTime = existingAnnouncement.StartTime
				announcementResource.EndTime = existingAnnouncement.EndTime
			}
		}

		var pendingApprovals []models.Approval
		_ = database.DB.Where(
			"resource_type = ? AND status = ? AND request_type = ?",
			"ANNOUNCEMENT",
			models.ApprovalStatus.PENDING,
			approval.RequestType,
		).Find(&pendingApprovals)

		for i := range pendingApprovals {
			var pendingAnnouncement AnnouncementApprovalResource
			err = json.Unmarshal([]byte(string(pendingApprovals[i].Resource)), &pendingAnnouncement)
			if err != nil {
				logger.Info(err)
				continue
			}

			if pendingAnnouncement.AnnouncementID == announcementResource.AnnouncementID &&
				pendingAnnouncement.Action == announcementResource.Action {
				c.JSON(http.StatusConflict, gin.H{
					"error":      "A pending approval already exists for this announcement action.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		var users []models.User
		var platformAdmins pq.StringArray

		database.DB.Where("type = ?", "ADMIN").Find(&users)

		for i := 0; i < len(users); i++ {
			if users[i].ID.String() == userID {
				continue
			}

			platformAdmins = append(platformAdmins, users[i].ID.String())
		}

		if len(platformAdmins) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "No other admin available to approve this announcement request.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		approval.RequestTo = platformAdmins

		title = "Raised A Request For " + approval.RequestType + "!!!"

		switch approval.RequestType {
		case models.RequestTypeRegistry.AnnouncementCreate:
			description = userEmail + " has raised a request to create announcement banner."
		case models.RequestTypeRegistry.AnnouncementUpdate:
			description = userEmail + " has raised a request to update announcement banner."
		case models.RequestTypeRegistry.AnnouncementDelete:
			description = userEmail + " has raised a request to delete announcement banner."
		default:
			description = userEmail + " has raised an announcement approval request."
		}

		resourceJSON, _ = json.Marshal(announcementResource)

	case "DOCUMENTATION":
		logger.Info("Documentation resource json:", string(approval.Resource))

		resourceJSON, documentationResource, err = validateAndNormalizeDocumentationApproval(approval.RequestType, approval.Resource)
		if err != nil {
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      err.Error(),
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		// Resource snapshot: for UPDATE/DELETE, pull the current DB values.
		if approval.RequestType == models.RequestTypeRegistry.DocumentationUpdate || approval.RequestType == models.RequestTypeRegistry.DocumentationDelete {
			type sectionSnapshot struct {
				Name            string
				Content         sql.NullString
				ParentSectionID *int64
			}

			var existingSection sectionSnapshot
			secDBRes := database.DB.Raw(
				"SELECT name, content, parent_section_id FROM sections WHERE id = $1",
				documentationResource.SectionID,
			).Scan(&existingSection)

			if secDBRes.Error != nil {
				logger.Errorln(secDBRes.Error)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      "Error while fetching existing documentation section.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if secDBRes.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Documentation section not found.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if approval.RequestType == models.RequestTypeRegistry.DocumentationUpdate {
				if existingSection.Content.Valid {
					documentationResource.OldContent = existingSection.Content.String
				} else {
					documentationResource.OldContent = ""
				}
			}

			if approval.RequestType == models.RequestTypeRegistry.DocumentationDelete {
				documentationResource.Name = existingSection.Name
				if existingSection.Content.Valid {
					documentationResource.Content = existingSection.Content.String
				} else {
					documentationResource.Content = ""
				}
				documentationResource.ParentSectionID = existingSection.ParentSectionID
			}

			resourceJSON, err = json.Marshal(documentationResource)
			if err != nil {
				logger.Errorln(err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      "Error while preparing documentation approval resource.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		var pendingApprovals []models.Approval
		_ = database.DB.Where(
			"resource_type = ? AND status = ? AND request_type = ?",
			"DOCUMENTATION",
			models.ApprovalStatus.PENDING,
			approval.RequestType,
		).Find(&pendingApprovals)

		for i := range pendingApprovals {
			var pendingDocumentation DocumentationApprovalResource
			err = json.Unmarshal([]byte(string(pendingApprovals[i].Resource)), &pendingDocumentation)
			if err != nil {
				logger.Info(err)
				continue
			}

			if documentationResource.Action == "CREATE" &&
				pendingDocumentation.Action == documentationResource.Action &&
				pendingDocumentation.Name == documentationResource.Name {
				c.JSON(http.StatusConflict, gin.H{
					"error":      "A pending approval already exists for this documentation section creation.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}

			if documentationResource.Action != "CREATE" &&
				pendingDocumentation.SectionID == documentationResource.SectionID &&
				pendingDocumentation.Action == documentationResource.Action {
				c.JSON(http.StatusConflict, gin.H{
					"error":      "A pending approval already exists for this documentation section action.",
					"success":    false,
					"request_id": reqID,
				})
				return
			}
		}

		var users []models.User
		var platformAdmins pq.StringArray

		database.DB.Where("type = ?", "ADMIN").Find(&users)

		for i := 0; i < len(users); i++ {
			if users[i].ID.String() == userID {
				continue
			}

			platformAdmins = append(platformAdmins, users[i].ID.String())
		}

		if len(platformAdmins) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "No other admin available to approve this documentation request.",
				"success":    false,
				"request_id": reqID,
			})
			return
		}

		approval.RequestTo = platformAdmins

		title = "Raised A Request For " + approval.RequestType + "!!!"

		switch approval.RequestType {
		case models.RequestTypeRegistry.DocumentationCreate:
			description = userEmail + " has raised a request to create documentation section."
		case models.RequestTypeRegistry.DocumentationUpdate:
			description = userEmail + " has raised a request to update documentation section."
		case models.RequestTypeRegistry.DocumentationDelete:
			description = userEmail + " has raised a request to delete documentation section."
		default:
			description = userEmail + " has raised a documentation approval request."
		}

	default:
		// Handle other resource types or invalid resource type error
		c.JSON(400, gin.H{
			"error":      "Provided Resource Type Is Not Valid!!!.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	var ProjectID string
	var ResourceID string
	var ResourceName string
	// Determine resource name dynamically based on resource type
	var logEvent *billingModel.AuditLog
	if approval.ResourceType == "PROJECT" {
		ProjectID = (resourceProject.ID).String()
		ResourceID = resourceProject.Name
		ResourceName = resourceProject.Name
	}

	if approval.ResourceType == "WORKSPACE" {
		ProjectID = resourceWorkspace.ProjectID
		ResourceID = resourceWorkspace.WorkspaceName
		ResourceName = resourceWorkspace.WorkspaceName

	}

	if approval.ResourceType == "ANNOUNCEMENT" {
		var announcementResource AnnouncementApprovalResource
		_ = json.Unmarshal(resourceJSON, &announcementResource)

		ProjectID = "ANNOUNCEMENT"
		ResourceID = announcementResource.AnnouncementID
		ResourceName = announcementResource.AnnouncementID
	}

	if approval.ResourceType == "DOCUMENTATION" {
		ProjectID = "DOCUMENTATION"

		if documentationResource.SectionID > 0 {
			ResourceID = fmt.Sprintf("%d", documentationResource.SectionID)
			ResourceName = fmt.Sprintf("%d", documentationResource.SectionID)
		} else {
			ResourceID = documentationResource.Name
			ResourceName = documentationResource.Name
		}
	}

	approval.Status = models.ApprovalStatus.PENDING
	approval.RequestFrom = getUserIDByEmail(approval.RequestFrom).String()
	for i := range approval.RequestTo {
		var notification models.Notification
		notification.ID = uuid.New()
		notification.ResourceID = approval.ID
		notification.RequestFrom = approval.RequestFrom
		notification.RequestTo = approval.RequestTo[i]
		notification.EventName = approval.RequestType
		notification.ResourceType = approval.ResourceType
		notification.ResourceName = ResourceName
		notification.ResourceStatus = approval.Status
		notification.Title = title
		notification.Description = description

		logger.Info("notification", notification)

		res := database.DB.Create(&notification)

		if res.RowsAffected == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error for approval with ID %d. No new approval created." + approval.ID.String(),
				"request_id": reqID,
			})
			return
		}
	}
	res := database.DB.Exec(
		`INSERT INTO approvals (id,request_from,request_to,request_type,is_existing_resource,resource_type,resource, created_at, updated_at, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		approval.ID, approval.RequestFrom, approval.RequestTo, approval.RequestType, approval.IsExistingResource, approval.ResourceType, resourceJSON, time.Now(), time.Now(), description)
	if res.RowsAffected == 0 {
		c.JSON(500, gin.H{
			"error":      "Error While Creating Approval.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	logEvent = &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "Approval",
		Action:            "Create",
		ProjectID:         ProjectID,
		ActionPerformedBy: userEmail,
		ResourceID:        ResourceID,
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          fmt.Sprintf("Project Created via Approval %s done by %s", approval.ApprovedBy, userEmail),
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)
	c.JSON(http.StatusOK, gin.H{
		"message":    "Approval Created Successfully.",
		"success":    true,
		"request_id": reqID,
	})
}

// GetAllApprovals lists all the approvals where user is involved
func GetAllApprovals(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Get Approvals API Invoked.")
	reqID, _ := c.Get("request_id")
	userID := c.GetString("user_id")
	var requestToUsers pq.StringArray
	var approvals []models.Approval
	approvalsResponse := []models.ApprovalResponse{}

	res := database.DB.Where("request_from = ? OR request_to @> ?", userID, append(requestToUsers, userID)).Find(&approvals)

	if res.RowsAffected != 0 {
		for i := 0; i < (len(approvals)); i++ {
			var approvedByEmail string
			if approvals[i].ApprovedBy != "" {
				approvedByEmail = getUserEmailByID(uuid.MustParse(approvals[i].ApprovedBy))
			}
			uniqueEmails := make(map[string]bool)
			var approvalToUserEmails []string
			for j := 0; j < (len(approvals[i].RequestTo)); j++ {
				email := getUserEmailByID(uuid.MustParse(approvals[i].RequestTo[j]))
				if !uniqueEmails[email] {
					uniqueEmails[email] = true
					approvalToUserEmails = append(approvalToUserEmails, email)
				}
			}
			approvalResponse := models.ApprovalResponse{
				ID:                 approvals[i].ID,
				CreatedAt:          approvals[i].CreatedAt,
				UpdatedAt:          approvals[i].UpdatedAt,
				RequestFrom:        getUserEmailByID(uuid.MustParse(approvals[i].RequestFrom)),
				RequestTo:          approvalToUserEmails,
				RequestType:        approvals[i].RequestType,
				Message:            approvals[i].Message,
				Status:             approvals[i].Status,
				ApprovedBy:         approvedByEmail,
				IsExistingResource: approvals[i].IsExistingResource,
				ResourceType:       approvals[i].ResourceType,
				Resource:           approvals[i].Resource,
				Description:        approvals[i].Description,
			}
			approvalsResponse = append(approvalsResponse, approvalResponse)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Approval Details Fetched Successfully...",
		"body":       approvalsResponse,
		"request_id": reqID,
	})
}

// UpdateApproval updates the actions done on approval
func UpdateApproval(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Update Approvals API Invoked.")
	reqID, _ := c.Get("request_id")
	userEmail := c.GetString("user_email")
	userID := c.GetString("user_id")
	approvalID := c.Param("approval_id")
	ip := c.ClientIP()
	var title string
	var description string
	var approvalPayload models.ApprovalPayload
	var foundApproval models.Approval
	foundApproval = models.Approval{}
	approvalResponse := models.ApprovalResponse{}

	err := c.ShouldBind(&approvalPayload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}

	// Use userID for comparison as used in the combined logic below
	if userEmail != approvalPayload.ApprovedBy {

		c.JSON(404, gin.H{
			"error":      "LoggedIn User Does Not Exists. Kindly Login With Correct Credentials",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	res := database.DB.Where("id = ?", approvalID).Find(&foundApproval)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   res.Error,
			"success": false,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"success":    false,
			"error":      "Approval Does Not Exists.",
			"request_id": reqID,
		})
		return
	}

	if foundApproval.ResourceType == "ANNOUNCEMENT" && foundApproval.RequestFrom == userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"error":      "You cannot approve your own announcement request.",
			"request_id": reqID,
		})
		return
	}

	if foundApproval.ResourceType == "DOCUMENTATION" && foundApproval.RequestFrom == userID {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"error":      "You cannot approve your own documentation request.",
			"request_id": reqID,
		})
		return
	}

	logger.Info("Found approval :", foundApproval)
	logger.Info("Approval payload:", approvalPayload)
	// Retaining the userID log line from the second function
	logger.Info("Logged In User", userID)
	if isAvailable(foundApproval.RequestTo, userID) == false {
		c.JSON(401, gin.H{
			"success":    false,
			"error":      "You Are Not Authorized To Perform This Action...",
			"request_id": reqID,
		})
		return
	}

	logger.Info("Updated Found approval :", foundApproval)

	if result := isAvailable([]string{models.ApprovalStatus.APPROVED, models.ApprovalStatus.REJECTED}, foundApproval.Status); result == true {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    "Approval Already " + foundApproval.Status + " by " + foundApproval.ApprovedBy,
			"request_id": reqID,
		})
		return
	}

	var resourceProject models.Project
	var resourceWorkspace Resource

	// Use a switch statement for handling resource types
	switch foundApproval.ResourceType {
	case "PROJECT":
		if foundApproval.RequestType == models.RequestTypeRegistry.ProjectOwnership {
			var ownershipResource ProjectOwnershipApprovalResource

			err = json.Unmarshal([]byte(string(foundApproval.Resource)), &ownershipResource)
			if err != nil {
				logger.Errorln(err)
				c.JSON(http.StatusBadRequest, gin.H{
					"success":    false,
					"error":      "Invalid project ownership transfer resource in approval.",
					"request_id": reqID,
				})
				return
			}

			var project models.Project
			res := database.DB.
				Where("id = ? AND is_deleted = ?", ownershipResource.ProjectID, false).
				First(&project)

			if res.Error != nil || res.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"error":      "Project Not Found",
					"request_id": reqID,
				})
				return
			}

			resourceProject = project

			if approvalPayload.Status == models.ApprovalStatus.APPROVED {
				err = applyProjectOwnershipTransferFromApproval(ownershipResource, foundApproval.ID, userEmail)
				if err != nil {
					logger.Errorln("Error while applying project ownership transfer:", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"success":    false,
						"error":      "Error while applying project ownership transfer: " + err.Error(),
						"request_id": reqID,
					})
					return
				}

				logEvent := &billingModel.AuditLog{
					ID:                uuid.MustParse(reqID.(string)),
					ResourceType:      "Project",
					Action:            "Ownership Transfer",
					ProjectID:         project.Name,
					ActionPerformedBy: getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
					ResourceID:        project.ID.String(),
					ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
					Comments:          fmt.Sprintf("Project ownership transferred via Approval %s done by %s", foundApproval.ID, userEmail),
					IP:                ip,
				}

				gormDb, _ := database.DB.DB()
				pgDB := commondb.GetPGDB(gormDb)
				auditlogs.CreateEvent(&pgDB, logEvent)
			}
		} else {
			err = json.Unmarshal([]byte(string(foundApproval.Resource)), &resourceProject)
			if err != nil {
				logger.Errorln(err)
				return
			}
			if approvalPayload.Status == models.ApprovalStatus.APPROVED {
				if len(resourceProject.Name) > 0 {
					if foundApproval.IsExistingResource == true {
						logger.Info("reuest type:", foundApproval.RequestType)
						if foundApproval.RequestType == models.RequestTypeRegistry.ProjectLead {
							logger.Info("Inside Lead")
							var foundUser models.User
							database.DB.Where("id = ?", foundApproval.RequestFrom).Find(&foundUser)
							err = RemoveFromProject(foundUser.ID, "Default")
							if err != nil {
								logger.Errorln("Error while removing user from project ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while removing user from project:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}
							err = addUserToProject(foundUser.ID, "DevOps_Lead", resourceProject.Name)
							if err != nil {
								logger.Errorln("Error while adding user to project ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while adding user to project:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}
						}
						if foundApproval.RequestType == models.RequestTypeRegistry.ProjectSubscription {
							logger.Info("Inside Subscription")
							isUserEntryAllowed := checkProjectLimit(resourceProject.ID)
							logger.Info("isUserEntryAllowed : ", isUserEntryAllowed)
							if isUserEntryAllowed == false {
								c.JSON(404, gin.H{
									"error":      "Mean While The Project Limit has been Exceeded, Please Contact Platform Admin",
									"success":    false,
									"request_id": reqID,
								})
								return
							}
							var foundUser models.User
							database.DB.Where("id = ?", foundApproval.RequestFrom).Find(&foundUser)
							err = RemoveFromProject(foundUser.ID, "Default")
							if err != nil {
								logger.Errorln("Error while removing user from project ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while removing user from project:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}
							err = addUserToProject(foundUser.ID, "DevOps", resourceProject.Name)
							if err != nil {
								logger.Errorln("Error while adding user to project ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while adding user to project:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}

							// Audit Log Event for User Addition to Project
							logEvent := &billingModel.AuditLog{
								ID:                uuid.MustParse(reqID.(string)),
								ResourceType:      "Project",
								Action:            "Add User",
								ProjectID:         resourceProject.Name,
								ActionPerformedBy: getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
								ResourceID:        (resourceProject.ID).String(),
								ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
								Comments:          fmt.Sprintf("User %s Added to Project via Approval %s done by %s", foundUser.Email, foundApproval.ID, userEmail),
								IP:                ip,
							}
							gormDb, _ := database.DB.DB()
							pgDB := commondb.GetPGDB(gormDb)
							auditlogs.CreateEvent(&pgDB, logEvent)
						}
						if foundApproval.RequestType == models.RequestTypeRegistry.ProjectUpdation {
							logger.Info("Inside Updation", resourceProject)
							if resourceProject.BuildQuota != 0 || resourceProject.VDIBuildQuota != 0 {
								err = updateBuildQuotaInProject(resourceProject)
								if err != nil {
									logger.Errorln("Error while updating build quota in project ", err)
									c.JSON(http.StatusInternalServerError, gin.H{
										"success":    false,
										"error":      "Error while updating build quota in project:- " + err.Error(),
										"request_id": reqID,
									})
									return
								}
								// Audit Log Event for Build Quota Update
								logEvent := &billingModel.AuditLog{
									ID:                uuid.MustParse(reqID.(string)),
									ResourceType:      "Project",
									Action:            "Update",
									ProjectID:         resourceProject.Name,
									ActionPerformedBy: getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
									ResourceID:        (resourceProject.ID).String(),
									ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
									Comments:          fmt.Sprintf("Build Quota Updated via Approval %s done by %s", foundApproval.ID, userEmail),
									IP:                ip,
								}
								gormDb, _ := database.DB.DB()
								pgDB := commondb.GetPGDB(gormDb)
								auditlogs.CreateEvent(&pgDB, logEvent)
							}
							if resourceProject.CPUQuota != 0 || resourceProject.MemoryQuota != 0 || resourceProject.StorageQuota != 0 || resourceProject.VDICPUQuota != 0 || resourceProject.VDIMemoryQuota != 0 || resourceProject.VDIStorageQuota != 0 {
								err = updateComputeQuotaInProject(resourceProject)
								if err != nil {
									logger.Errorln("Error while updating compute quota in project ", err)
									c.JSON(http.StatusInternalServerError, gin.H{
										"success":    false,
										"error":      "Error while updating compute quota in project:- " + err.Error(),
										"request_id": reqID,
									})
									return
								}
								// Audit Log Event for Compute Quota Update
								logEvent := &billingModel.AuditLog{
									ID:                uuid.MustParse(reqID.(string)),
									ResourceType:      "Project",
									Action:            "Update",
									ProjectID:         resourceProject.Name,
									ActionPerformedBy: getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
									ResourceID:        (resourceProject.ID).String(),
									ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
									Comments:          fmt.Sprintf("Compute Quota Updated via Approval %s done by %s", foundApproval.ID, userEmail),
									IP:                ip,
								}
								gormDb, _ := database.DB.DB()
								pgDB := commondb.GetPGDB(gormDb)
								auditlogs.CreateEvent(&pgDB, logEvent)
							}
							err = updateQuotaInNamespace(logger, resourceProject)
							if err != nil {
								logger.Errorln("Error while updating resorce quota in Namespace ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while updating resorce quota in Namespace:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}
						}
					}
					if foundApproval.IsExistingResource == false {
						var foundUser models.User
						var project models.Project
						var foundProject models.Project
						database.DB.Where("id = ?", foundApproval.RequestFrom).Find(&foundUser)
						err = RemoveFromProject(foundUser.ID, "Default")
						if err != nil {
							logger.Errorln("Error while removing user from project ", err)
							c.JSON(http.StatusInternalServerError, gin.H{
								"success":    false,
								"error":      "Error while removing user from project:- " + err.Error(),
								"request_id": reqID,
							})
							return
						}
						project = resourceProject
						project.ID = uuid.New()
						logger.Info("Project detils :", project)

						res := database.DB.Where("name = ?", project.Name).Find(&foundProject)
						if res.RowsAffected == 0 {
							_, err = onboardProject(project.Name, logger, foundApproval.RequestFrom, project)
							if err != nil {
								logger.Errorln("Error while onboarding project ", err)
								c.JSON(http.StatusInternalServerError, gin.H{
									"success":    false,
									"error":      "Error while onboarding project:- " + err.Error(),
									"request_id": reqID,
								})
								return
							}
							res = database.DB.Create(&project)
							if res.Error != nil {
								c.JSON(http.StatusInternalServerError, gin.H{
									"error":      res.Error,
									"request_id": reqID,
								})
								return
							}
							if res.RowsAffected == 0 {
								c.JSON(http.StatusBadRequest, gin.H{
									"error":      "Error Occured While Creating Project",
									"success":    false,
									"request_id": reqID,
								})
								return
							}

							// Audit Log Event for Project Creation
							logEvent := &billingModel.AuditLog{
								ID:                uuid.MustParse(reqID.(string)),
								ResourceType:      "Project",
								Action:            "Create",
								ProjectID:         resourceProject.Name,
								ActionPerformedBy: getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
								ResourceID:        (resourceProject.ID).String(),
								ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
								Comments:          fmt.Sprintf("Project Created via Approval %s done by %s", foundApproval.ID, userEmail),
								IP:                ip,
							}
							gormDb, _ := database.DB.DB()
							pgDB := commondb.GetPGDB(gormDb)
							auditlogs.CreateEvent(&pgDB, logEvent)
						} else {
							c.JSON(409, gin.H{
								"success":    false,
								"error":      "Project is created with the name " + project.Name,
								"request_id": reqID,
							})
							return
						}
					}
				}
			}
		}
	case "WORKSPACE":
		err = json.Unmarshal([]byte(string(foundApproval.Resource)), &resourceWorkspace)
		if err != nil {
			logger.Errorln(err)
			return
		}

		var adminRequestID = uuid.New()
		if approvalPayload.Status == models.ApprovalStatus.APPROVED {
			if foundApproval.RequestType == models.RequestTypeRegistry.WorkspaceAdmin && len(resourceWorkspace.WorkspaceName) > 0 {
				res := database.DB.Exec(
					`INSERT INTO admin_requests (id,workspace_name, workspace_type, project_id,  duration, status, requested_at,requested_by,approved_by,approved_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8,$9,$10)`,
					adminRequestID, resourceWorkspace.WorkspaceName, resourceWorkspace.WorkspaceType, resourceWorkspace.ProjectID, resourceWorkspace.Duration, "GRANT_PENDING", foundApproval.CreatedAt, foundApproval.RequestFrom, userID, time.Now())
				if res.RowsAffected == 0 {
					c.JSON(500, gin.H{
						"error":      "Error While Creating Approval.",
						"success":    false,
						"request_id": reqID,
					})
					return
				}
			}
			if foundApproval.RequestType == models.RequestTypeRegistry.WorkspaceDeletion && len(resourceWorkspace.WorkspaceName) > 0 {
				// workspace_ms verifies the approval is APPROVED before deleting, so the status
				// must be persisted before calling it (the general status UPDATE below runs too late).
				// Conditional on the current status so that, across replicas, only one of two
				// concurrent approvals wins and triggers the delete.
				res := database.DB.Exec(
					`UPDATE approvals SET message = $1, status = $2, approved_by = $3, updated_at = $4 WHERE id = $5 AND status = $6`,
					approvalPayload.Message, approvalPayload.Status, getUserIDByEmail(approvalPayload.ApprovedBy), time.Now(), approvalID, foundApproval.Status)
				if res.Error != nil {
					c.JSON(500, gin.H{
						"error":      "Error While Updating Approval.",
						"success":    false,
						"request_id": reqID,
					})
					return
				}
				if res.RowsAffected == 0 {
					c.JSON(http.StatusConflict, gin.H{
						"error":      "Approval was already processed by another request.",
						"success":    false,
						"request_id": reqID,
					})
					return
				}

				// If the delete can't be carried out, put the approval back to its previous status so
				// it doesn't read as APPROVED for a workspace that still exists, and so it can be retried.
				failDeletion := func(status int, msg string) {
					logger.Errorln(msg)
					database.DB.Exec(
						`UPDATE approvals SET status = $1, approved_by = $2, updated_at = $3 WHERE id = $4`,
						foundApproval.Status, foundApproval.ApprovedBy, time.Now(), approvalID)
					c.JSON(status, gin.H{
						"error":      msg,
						"success":    false,
						"request_id": reqID,
					})
				}

				tokenString := c.GetHeader("Authorization")

				envURL := os.Getenv("ENV_BASE_URL")
				baseURL := fmt.Sprintf("%s/api/v1/workspaces/%s", envURL, resourceWorkspace.WorkspaceName)

				deleteURL, err := url.Parse(baseURL)
				if err != nil {
					failDeletion(http.StatusInternalServerError, "Invalid workspace delete URL: "+err.Error())
					return
				}
				q := deleteURL.Query()
				q.Set("approval_id", (foundApproval.ID).String())
				deleteURL.RawQuery = q.Encode()

				req, err := http.NewRequest(http.MethodDelete, deleteURL.String(), nil)
				if err != nil {
					failDeletion(http.StatusInternalServerError, "Error creating workspace delete request: "+err.Error())
					return
				}
				logger.Info("workspace delete url :", deleteURL)
				req.Header.Set("Authorization", tokenString)
				req.Header.Set("Accept", "application/json")

				// Transport with InsecureSkipVerify enabled
				transport := &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}

				client := &http.Client{Transport: transport}
				resp, err := client.Do(req)
				if err != nil {
					failDeletion(http.StatusBadGateway, "Error calling workspace service for deletion: "+err.Error())
					return
				}
				defer resp.Body.Close()

				body, _ := io.ReadAll(resp.Body)
				logger.Infof("Workspace delete status: %d", resp.StatusCode)
				logger.Infof("Workspace delete response: %s", string(body))

				if msg := workspaceDeleteError(resp.StatusCode, body); msg != "" {
					failDeletion(resp.StatusCode, msg)
					return
				}
			}
		}

	case "ANNOUNCEMENT":
		var announcementResource AnnouncementApprovalResource

		err = json.Unmarshal([]byte(string(foundApproval.Resource)), &announcementResource)
		if err != nil {
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success":    false,
				"error":      "Invalid announcement resource in approval.",
				"request_id": reqID,
			})
			return
		}

		err = notifyDocsMSAnnouncementAction(
			c,
			foundApproval.ID.String(),
			announcementResource,
			approvalPayload.Status,
		)
		if err != nil {
			logger.Errorln("Error while notifying docs_ms for announcement action:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error while notifying docs_ms for announcement action: " + err.Error(),
				"request_id": reqID,
			})
			return
		}

	case "DOCUMENTATION":
		var documentationResource DocumentationApprovalResource

		err = json.Unmarshal([]byte(string(foundApproval.Resource)), &documentationResource)
		if err != nil {
			logger.Errorln(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"success":    false,
				"error":      "Invalid documentation resource in approval.",
				"request_id": reqID,
			})
			return
		}

		err = notifyDocsMSDocumentationAction(
			c,
			foundApproval.ID.String(),
			documentationResource,
			approvalPayload.Status,
		)
		if err != nil {
			logger.Errorln("Error while notifying docs_ms for documentation action:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error while notifying docs_ms for documentation action: " + err.Error(),
				"request_id": reqID,
			})
			return
		}

	default:
		// Handle unexpected resource type
		c.JSON(400, gin.H{
			"error":      "Invalid Resource Type Found in Approval.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	var usersToSendNotifications []string
	var resourceName string
	if foundApproval.ResourceType == "PROJECT" {
		resourceName = resourceProject.Name
	} else {
		resourceName = "Default"
	}

	if foundApproval.ResourceType == "WORKSPACE" {
		resourceName = resourceWorkspace.ProjectID
	}

	if foundApproval.ResourceType == "ANNOUNCEMENT" {
		var announcementResource AnnouncementApprovalResource
		_ = json.Unmarshal([]byte(string(foundApproval.Resource)), &announcementResource)
		resourceName = announcementResource.AnnouncementID
	}

	if foundApproval.ResourceType == "DOCUMENTATION" {
		var documentationResource DocumentationApprovalResource
		_ = json.Unmarshal([]byte(string(foundApproval.Resource)), &documentationResource)

		if documentationResource.SectionID > 0 {
			resourceName = fmt.Sprintf("%d", documentationResource.SectionID)
		} else {
			resourceName = documentationResource.Name
		}
	}

	usersToSendNotifications = append(usersToSendNotifications, foundApproval.RequestFrom)
	usersToSendNotifications = append(usersToSendNotifications, foundApproval.RequestTo...)
	for i := 0; i < len(usersToSendNotifications); i++ {
		if usersToSendNotifications[i] == userID {
			usersToSendNotifications = append(usersToSendNotifications[:i], usersToSendNotifications[i+1:]...)
			i--
		}
	}
	// Ensure unique users in the notification list
	uniqueUsersToSendNotifications := make(map[string]bool)
	for _, user := range usersToSendNotifications {
		uniqueUsersToSendNotifications[user] = true
	}
	title = "Approval Updated With Status " + approvalPayload.Status
	description = userEmail + " has responded to the approval with ID " + foundApproval.ID.String()
	for userID := range uniqueUsersToSendNotifications {
		var existingNotification models.Notification
		err := database.DB.Where("resource_id = ? AND request_from = ? AND request_to = ? AND event_name = ?",
			foundApproval.ID, foundApproval.RequestFrom, userID, foundApproval.RequestType).First(&existingNotification).Error

		if err == nil {
			logger.Info("Duplicate notification detected, skipping creation for:", userID)
			continue
		}

		var notification models.Notification
		notification.ID = uuid.New()
		notification.ResourceID = foundApproval.ID
		notification.RequestFrom = foundApproval.RequestFrom
		notification.RequestTo = userID
		notification.EventName = foundApproval.RequestType
		notification.ResourceType = foundApproval.ResourceType
		notification.ResourceName = resourceName
		notification.ResourceStatus = approvalPayload.Status
		notification.Title = title
		notification.Description = description

		logger.Info("notification", notification)

		res := database.DB.Create(&notification)

		if res.RowsAffected == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      err.Error() + "for approval with ID %d" + foundApproval.ID.String(),
				"request_id": reqID,
			})
			return
		}
	}
	updatedAt := time.Now()
	// Use the userID directly which was retrieved from the token and used for comparison earlier
	approvedBy := getUserIDByEmail(approvalPayload.ApprovedBy)
	res = database.DB.Exec(
		`UPDATE approvals SET message = $1, status = $2, approved_by = $3, updated_at = $4 WHERE id = $5`, approvalPayload.Message, approvalPayload.Status, approvedBy, updatedAt, approvalID)
	if res.RowsAffected == 0 {
		c.JSON(500, gin.H{
			"error":      "Error While Creating Approval.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	var requestToUsers pq.StringArray
	for i := range foundApproval.RequestTo {
		requestToUsers = append(requestToUsers, getUserEmailByID(uuid.MustParse(foundApproval.RequestTo[i])))
	}
	// Audit Log Event for User Addition to Project
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "Approval",
		Action:            "Update",
		ProjectID:         resourceName,
		ActionPerformedBy: approvalPayload.ApprovedBy,
		ResourceID:        foundApproval.ID.String(),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          approvalPayload.Status,
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)

	approvalResponse = models.ApprovalResponse{
		ID:                 foundApproval.ID,
		CreatedAt:          foundApproval.CreatedAt,
		UpdatedAt:          foundApproval.UpdatedAt,
		RequestFrom:        getUserEmailByID(uuid.MustParse(foundApproval.RequestFrom)),
		RequestTo:          requestToUsers,
		RequestType:        foundApproval.RequestType,
		Message:            approvalPayload.Message,
		Status:             approvalPayload.Status,
		ApprovedBy:         approvalPayload.ApprovedBy,
		IsExistingResource: foundApproval.IsExistingResource,
		ResourceType:       foundApproval.ResourceType,
		Resource:           foundApproval.Resource,
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Approval Successfully Updated...",
		"body":       approvalResponse,
		"request_id": reqID,
	})
}

func isAvailable(list []string, str string) bool {
	// iterate using the for loop
	for i := 0; i < len(list); i++ {
		if list[i] == str {
			return true
		}
	}
	return false
}

func isProjectExists(projectName string) bool {
	var projects []models.Project
	_ = database.DB.Find(&projects)
	var projectNames []string
	for i := 0; i < (len(projects)); i++ {
		projectNames = append(projectNames, projects[i].Name)
	}

	result := isAvailable(projectNames, projectName)
	return result
}

// RemoveFromProject helps to remove user from a specific project.
func RemoveFromProject(requestFromUserID uuid.UUID, projectName string) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	var project models.Project
	_ = database.DB.Where("name = ?", projectName).Find(&project)
	logger.Info("removing user id from default project member list.")
	logger.Info("project name :", projectName)
	logger.Info("project name :", project.Name)
	logger.Info("project details :", project)
	for i := 0; i < len(project.ProjectMembers); i++ {
		logger.Info("project member :", project.ProjectMembers[i])
		logger.Info("user id :", requestFromUserID.String())
		if project.ProjectMembers[i] == requestFromUserID.String() {
			project.ProjectMembers = append(project.ProjectMembers[:i], project.ProjectMembers[i+1:]...)
			res := database.DB.Model(&project).Where("id = ?", project.ID).Updates(project)
			if res.Error != nil {
				logger.Errorln("Error while removing user from existing project  ", res.Error)
				return res.Error
			}
		}
	}
	return nil
}
func updateBuildQuotaInProject(project models.Project) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))

	res := database.DB.Exec(
		`UPDATE projects SET build_quota=$1, vdi_build_quota=$2 WHERE name = $3 AND is_deleted = $4`, project.BuildQuota, project.VDIBuildQuota, project.Name, false)
	if res.RowsAffected == 0 {
		logger.Errorln("Error while updating compute quota in existing project  ", res.Error)
		return res.Error
	}
	return nil
}
func updateComputeQuotaInProject(project models.Project) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))

	res := database.DB.Exec(
		`UPDATE projects SET cpu_quota = $1, memory_quota = $2, storage_quota = $3, vdi_cpu_quota = $4, vdi_memory_quota = $5, vdi_storage_quota = $6 WHERE name = $7 AND is_deleted = $8`, project.CPUQuota, project.MemoryQuota, project.StorageQuota, project.VDICPUQuota, project.VDIMemoryQuota, project.VDIStorageQuota, project.Name, false)
	if res.RowsAffected == 0 {
		logger.Errorln("Error while updating compute quota in existing project  ", res.Error)
		return res.Error
	}
	return nil
}

func addUserToProject(userID uuid.UUID, userType string, projectName string) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	var project models.Project
	_ = database.DB.Where("name = ?", projectName).Find(&project)
	if userType == "DevOps" {
		project.ProjectMembers = append(project.ProjectMembers, userID.String())
	}
	if userType == "DevOps_Lead" {
		// Check if the user is already in ProjectMembers
		for i, memberID := range project.ProjectMembers {
			if memberID == userID.String() {
				// Remove user from ProjectMembers
				project.ProjectMembers = append(project.ProjectMembers[:i], project.ProjectMembers[i+1:]...)
				break
			}
		}
		// Add user to ProjectAdmins
		project.ProjectAdmins = append(project.ProjectAdmins, userID.String())
	}
	res := database.DB.Model(&project).Where("name = ?", projectName).Updates(project)
	if res.Error != nil {
		logger.Errorln("Error while removing user from existing project  ", res.Error)
		return res.Error
	}
	return nil
}
func isProjectRecentlyDeleted(projectname string) bool {
	var project models.Project
	_ = database.DB.Where("name = ?", projectname).Find(&project)
	return project.IsDeleted == true
}
func checkProjectLimit(projectID uuid.UUID) bool {
	var project models.Project
	_ = database.DB.Where("id = ?", projectID).Find(&project)
	existingCount := len(project.ProjectAdmins) + len(project.ProjectMembers)
	return project.UserQuota > existingCount
}

// workspaceDeleteError returns a user-facing error message when workspace_ms did not accept
// the delete request (non-2xx), including workspace_ms's own "error" field when present, or ""
// when the delete was accepted.
func workspaceDeleteError(statusCode int, body []byte) string {
	if statusCode >= 200 && statusCode < 300 {
		return ""
	}
	msg := fmt.Sprintf("Workspace deletion failed (status %d)", statusCode)
	var wsResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &wsResp) == nil && wsResp.Error != "" {
		msg += ": " + wsResp.Error
	}
	return msg
}

func getUserIDByEmail(userEmail string) uuid.UUID {
	var user models.User
	_ = database.DB.Where("email = ?", userEmail).Find(&user)
	return user.ID
}

func getUserEmailByID(userID uuid.UUID) string {
	var user models.User
	_ = database.DB.Where("id = ?", userID).Find(&user)
	return user.Email
}
func updateQuotaInNamespace(logger *zap.SugaredLogger, project models.Project) error {
	gitHost := os.Getenv("GIT_HOST")
	if gitHost == "" {
		logger.Error("Environment variable GIT_HOST is not set")
	}
	owner := os.Getenv("GIT_OWNER")
	if owner == "" {
		logger.Error("Environment variable GIT_OWNER is not set")
	}
	repo := os.Getenv("GIT_REPO")
	if repo == "" {
		logger.Error("Environment variable GIT_REPO is not set")
	}
	branch := os.Getenv("GIT_BRANCH")
	if branch == "" {
		logger.Error("Environment variable GIT_BRANCH is not set")
	}
	// Get GitHub client using centralized gitclient package
	httpClient, err := gitclient.GetGitClient()
	if err != nil {
		logger.Errorln("Error creating GitHub client: ", err)
	}
	ghClient := gitclient.GetGitHubClient(httpClient)
	var buildQuota = int(project.BuildQuota + project.VDIBuildQuota)
	var nsCPU = int(project.CPUQuota+project.VDICPUQuota) + buildQuota

	var nsMemory = int(project.MemoryQuota+project.VDIMemoryQuota) + buildQuota
	nsquota := fmt.Sprintf(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: cpu-mem-quota
spec:
  hard:
    cpu: "%d"
    memory: "%dGi"`, nsCPU, nsMemory)
	entries := []github.TreeEntry{
		{
			Path:    github.String("tenants/" + project.Name + "/resourcequota.yaml"),
			Type:    github.String("blob"),
			Content: github.String(nsquota),
			Mode:    github.String("100644"),
		},
	}
	ghClient.CommitContent(entries, "Updated quota for "+project.Name)
	return err
}

func notifyDocsMSAnnouncementAction(
	c *gin.Context,
	approvalID string,
	resource AnnouncementApprovalResource,
	status string,
) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))

	baseURL := strings.TrimRight(os.Getenv("DOCS_MS_BASE_URL"), "/")
	if baseURL == "" {
		env := os.Getenv("APP_ENV")

		switch env {
		case "local":
			baseURL = "http://localhost:8099"
		case "dev":
			baseURL = "https://internal.vcollab-tech-in.vodafone.com"
		case "qa":
			baseURL = "https://preprod.vcollab-tech-in.vodafone.com"
		case "prod":
			baseURL = "https://vcollab-tech-in.vodafone.com"
		default:
			return fmt.Errorf("DOCS_MS_BASE_URL environment variable is not set and APP_ENV is invalid: %s", env)
		}
	}

	var method string
	var endpoint string

	if status == models.ApprovalStatus.APPROVED {
		switch resource.Action {
		case "CREATE":
			method = http.MethodPost
			endpoint = fmt.Sprintf("%s/api/v1/announcements", baseURL)

		case "UPDATE":
			method = http.MethodPut
			endpoint = fmt.Sprintf("%s/api/v1/announcements/%s", baseURL, resource.AnnouncementID)

		case "DELETE":
			method = http.MethodDelete
			endpoint = fmt.Sprintf("%s/api/v1/announcements/%s", baseURL, resource.AnnouncementID)

		default:
			return fmt.Errorf("invalid announcement action: %s", resource.Action)
		}
	} else if status == models.ApprovalStatus.REJECTED {
		return nil
	} else {
		return fmt.Errorf("invalid approval status: %s", status)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	q := parsedURL.Query()
	q.Set("approval_id", approvalID)
	parsedURL.RawQuery = q.Encode()

	var body io.Reader

	if resource.Action == "CREATE" || resource.Action == "UPDATE" {
		reqBody := gin.H{
			"heading":           resource.Heading,
			"subheading":        resource.Subheading,
			"announcement_type": resource.AnnouncementType,
			"start_time":        resource.StartTime,
			"end_time":          resource.EndTime,
		}

		reqBodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}

		body = bytes.NewBuffer(reqBodyBytes)
	}

	req, err := http.NewRequest(method, parsedURL.String(), body)
	if err != nil {
		return err
	}

	tokenString := c.GetHeader("Authorization")
	req.Header.Set("Authorization", tokenString)
	req.Header.Set("Accept", "application/json")

	if resource.Action == "CREATE" || resource.Action == "UPDATE" {
		req.Header.Set("Content-Type", "application/json")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	logger.Infof("docs_ms announcement action url: %s", parsedURL.String())
	logger.Infof("docs_ms announcement action method: %s", method)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	logger.Infof("docs_ms announcement action status: %d", resp.StatusCode)
	logger.Infof("docs_ms announcement action response: %s", string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("docs_ms action failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func validateAnnouncementApprovalPayload(resource AnnouncementApprovalResource) error {
	heading := strings.TrimSpace(resource.Heading)
	subheading := strings.TrimSpace(resource.Subheading)
	announcementType := strings.TrimSpace(resource.AnnouncementType)

	if heading == "" {
		return fmt.Errorf("heading is required")
	}

	if len(heading) > 120 {
		return fmt.Errorf("heading must not exceed 120 characters")
	}

	if len(subheading) > 300 {
		return fmt.Errorf("subheading must not exceed 300 characters")
	}

	if announcementType == "" {
		return fmt.Errorf("announcement_type is required")
	}

	if !isValidAnnouncementType(announcementType) {
		return fmt.Errorf("announcement_type must be one of: note, tip, important, warning, caution")
	}

	if resource.StartTime.IsZero() {
		return fmt.Errorf("start_time is required")
	}

	if resource.EndTime.IsZero() {
		return fmt.Errorf("end_time is required")
	}

	if !resource.StartTime.Before(resource.EndTime) {
		return fmt.Errorf("start_time must be before end_time")
	}

	if resource.EndTime.Before(time.Now()) {
		return fmt.Errorf("end_time must be in future")
	}

	return nil
}

// isValidAnnouncementType only accepts given announcement_type
func isValidAnnouncementType(announcementType string) bool {
	switch announcementType {
	case "note", "tip", "important", "warning", "caution":
		return true
	default:
		return false
	}
}

// validateDocumentationApprovalPayload validates documentation approval resource before storing it.
func validateDocumentationApprovalPayload(resource DocumentationApprovalResource) error {
	action := strings.TrimSpace(resource.Action)

	if action == "" {
		return fmt.Errorf("documentation action is required")
	}

	switch action {
	case "CREATE":
		if strings.TrimSpace(resource.Name) == "" {
			return fmt.Errorf("section name is required")
		}

		if strings.TrimSpace(resource.Content) == "" {
			return fmt.Errorf("content is required")
		}

	case "UPDATE":
		if resource.SectionID <= 0 {
			return fmt.Errorf("section_id is required for update")
		}

		if strings.TrimSpace(resource.Name) == "" {
			return fmt.Errorf("section name is required")
		}

		if strings.TrimSpace(resource.Content) == "" {
			return fmt.Errorf("content is required")
		}

	case "DELETE":
		if resource.SectionID <= 0 {
			return fmt.Errorf("section_id is required for delete")
		}

	default:
		return fmt.Errorf("invalid documentation action: %s", resource.Action)
	}

	for _, image := range resource.Images {
		if strings.TrimSpace(image.OriginalFilename) == "" {
			return fmt.Errorf("image original_filename is required")
		}

		if strings.TrimSpace(image.MimeType) == "" {
			return fmt.Errorf("image mime_type is required")
		}

		if strings.TrimSpace(image.Placeholder) == "" {
			return fmt.Errorf("image placeholder is required")
		}

		if strings.TrimSpace(image.ImageData) == "" {
			return fmt.Errorf("image_data is required")
		}

		if _, err := base64.StdEncoding.DecodeString(image.ImageData); err != nil {
			return fmt.Errorf("invalid base64 image_data for image %s", image.OriginalFilename)
		}

		if !strings.Contains(resource.Content, image.Placeholder) {
			return fmt.Errorf("image placeholder is missing from content")
		}
	}

	return nil
}

// validateAndNormalizeDocumentationApproval validates request type/action mapping and returns normalized resource JSON.
func validateAndNormalizeDocumentationApproval(approvalRequestType string, rawResource json.RawMessage) ([]byte, DocumentationApprovalResource, error) {
	var documentationResource DocumentationApprovalResource

	if err := json.Unmarshal([]byte(string(rawResource)), &documentationResource); err != nil {
		return nil, documentationResource, fmt.Errorf("invalid documentation resource payload")
	}

	validDocumentationRequestTypes := []string{
		models.RequestTypeRegistry.DocumentationCreate,
		models.RequestTypeRegistry.DocumentationUpdate,
		models.RequestTypeRegistry.DocumentationDelete,
	}

	if !isAvailable(validDocumentationRequestTypes, approvalRequestType) {
		return nil, documentationResource, fmt.Errorf("invalid documentation request type")
	}

	validActionForRequest := map[string]string{
		models.RequestTypeRegistry.DocumentationCreate: "CREATE",
		models.RequestTypeRegistry.DocumentationUpdate: "UPDATE",
		models.RequestTypeRegistry.DocumentationDelete: "DELETE",
	}

	expectedAction := validActionForRequest[approvalRequestType]

	if documentationResource.Action != expectedAction {
		return nil, documentationResource, fmt.Errorf("documentation action does not match request type")
	}

	if err := validateDocumentationApprovalPayload(documentationResource); err != nil {
		return nil, documentationResource, err
	}

	resourceJSON, err := json.Marshal(documentationResource)
	if err != nil {
		return nil, documentationResource, err
	}

	return resourceJSON, documentationResource, nil
}

// notifyDocsMSDocumentationAction calls docs_ms section mutation endpoints after documentation approval is approved.
func notifyDocsMSDocumentationAction(
	c *gin.Context,
	approvalID string,
	resource DocumentationApprovalResource,
	status string,
) error {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))

	if status == models.ApprovalStatus.REJECTED {
		return nil
	}

	if status != models.ApprovalStatus.APPROVED {
		return fmt.Errorf("invalid approval status: %s", status)
	}

	baseURL := strings.TrimRight(os.Getenv("DOCS_MS_BASE_URL"), "/")
	if baseURL == "" {
		env := os.Getenv("APP_ENV")

		switch env {
		case "local":
			baseURL = "http://localhost:8099"
		case "dev":
			baseURL = "https://internal.vcollab-tech-in.vodafone.com"
		case "qa":
			baseURL = "https://preprod.vcollab-tech-in.vodafone.com"
		case "prod":
			baseURL = "https://vcollab-tech-in.vodafone.com"
		default:
			return fmt.Errorf("DOCS_MS_BASE_URL environment variable is not set and APP_ENV is invalid: %s", env)
		}
	}

	var method string
	var endpoint string
	var body io.Reader
	var contentType string

	switch resource.Action {
	case "CREATE":
		method = http.MethodPost
		endpoint = fmt.Sprintf("%s/api/v1/sections", baseURL)

		reqBody, reqContentType, err := buildDocumentationMultipartBody(resource)
		if err != nil {
			return err
		}

		body = reqBody
		contentType = reqContentType

	case "UPDATE":
		if resource.SectionID <= 0 {
			return fmt.Errorf("section_id is required for update")
		}

		method = http.MethodPut
		endpoint = fmt.Sprintf("%s/api/v1/sections/%d", baseURL, resource.SectionID)

		reqBody, reqContentType, err := buildDocumentationMultipartBody(resource)
		if err != nil {
			return err
		}

		body = reqBody
		contentType = reqContentType

	case "DELETE":
		if resource.SectionID <= 0 {
			return fmt.Errorf("section_id is required for delete")
		}

		method = http.MethodDelete
		endpoint = fmt.Sprintf("%s/api/v1/sections/%d", baseURL, resource.SectionID)

	default:
		return fmt.Errorf("invalid documentation action: %s", resource.Action)
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	q := parsedURL.Query()
	q.Set("approval_id", approvalID)
	parsedURL.RawQuery = q.Encode()

	req, err := http.NewRequest(method, parsedURL.String(), body)
	if err != nil {
		return err
	}

	tokenString := c.GetHeader("Authorization")
	req.Header.Set("Authorization", tokenString)
	req.Header.Set("Accept", "application/json")

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	logger.Infof("docs_ms documentation action url: %s", parsedURL.String())
	logger.Infof("docs_ms documentation action method: %s", method)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	logger.Infof("docs_ms documentation action status: %d", resp.StatusCode)
	logger.Infof("docs_ms documentation action response: %s", string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("docs_ms documentation action failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// buildDocumentationMultipartBody converts approval.resource JSON into multipart form expected by docs_ms section endpoints.
func buildDocumentationMultipartBody(resource DocumentationApprovalResource) (io.Reader, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)

	if err := writer.WriteField("name", resource.Name); err != nil {
		return nil, "", err
	}

	if err := writer.WriteField("content", resource.Content); err != nil {
		return nil, "", err
	}

	if resource.ParentSectionID != nil {
		if err := writer.WriteField("parent_section_id", strconv.FormatInt(*resource.ParentSectionID, 10)); err != nil {
			return nil, "", err
		}
	}

	for _, image := range resource.Images {
		imageBytes, err := base64.StdEncoding.DecodeString(image.ImageData)
		if err != nil {
			return nil, "", err
		}

		if err := writer.WriteField("placeholders", image.Placeholder); err != nil {
			return nil, "", err
		}

		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="images"; filename="%s"`, escapeQuotes(image.OriginalFilename)))
		partHeader.Set("Content-Type", image.MimeType)

		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return nil, "", err
		}

		if _, err := part.Write(imageBytes); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return &buffer, writer.FormDataContentType(), nil
}

// escapeQuotes safely escapes quotes used in multipart filenames.
func escapeQuotes(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func isBeneficiaryProjectAdmin(project models.Project, beneficiaryEmail string) bool {
	beneficiaryEmail = strings.TrimSpace(strings.ToLower(beneficiaryEmail))
	beneficiaryUserID := getUserIDByEmail(beneficiaryEmail).String()

	for _, admin := range project.ProjectAdmins {
		admin = strings.TrimSpace(admin)

		if strings.EqualFold(admin, beneficiaryEmail) {
			return true
		}

		if beneficiaryUserID != "" && admin == beneficiaryUserID {
			return true
		}
	}

	return false
}

func applyProjectOwnershipTransferFromApproval(resource ProjectOwnershipApprovalResource, approvalID uuid.UUID, approvedBy string) error {
	resource.ProjectID = strings.TrimSpace(resource.ProjectID)
	resource.TransferType = strings.TrimSpace(strings.ToLower(resource.TransferType))
	resource.Beneficiary = strings.TrimSpace(strings.ToLower(resource.Beneficiary))

	if resource.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	if !isValidUUID(resource.ProjectID) {
		return fmt.Errorf("invalid Project ID, Project ID should be UUID")
	}

	if resource.TransferType == "" {
		return fmt.Errorf("transfer_type is required")
	}

	if resource.Beneficiary == "" {
		return fmt.Errorf("beneficiary is required")
	}

	// Common validation for both permanent and temporary transfer.
	var project models.Project
	res := database.DB.
		Where("id = ? AND is_deleted = ?", resource.ProjectID, false).
		First(&project)

	if res.RowsAffected == 0 {
		return fmt.Errorf("project not found")
	}

	if res.Error != nil {
		return fmt.Errorf("unable to fetch project details")
	}

	if strings.EqualFold(strings.TrimSpace(project.AdminEmail), resource.Beneficiary) {
		return fmt.Errorf("project ownership cannot be transferred to the current owner")
	}

	if !isBeneficiaryProjectAdmin(project, resource.Beneficiary) {
		return fmt.Errorf("beneficiary must be one of the project admins")
	}

	switch resource.TransferType {
	case "permanent":
		var delegationCount int64
		database.DB.Model(&models.ProjectAdminDelegation{}).Where("project_id = ? AND status IN ?", project.ID, []string{models.ProjectAdminDelegationStatus.PENDING, models.ProjectAdminDelegationStatus.APPROVED}).Count(&delegationCount)
		if delegationCount > 0 {
			return fmt.Errorf("permanent ownership transfer cannot be completed while a temporary delegation exists")
		}

		// Permanent transfer updates project's AdminEmail immediately after approval.
		updateRes := database.DB.
			Model(&project).
			Update("admin_email", resource.Beneficiary)

		if updateRes.Error != nil {
			return fmt.Errorf("unable to transfer project ownership")
		}

		if updateRes.RowsAffected == 0 {
			return fmt.Errorf("project ownership could not be transferred")
		}

		return nil

	case "temporary":
		if err := validateTemporaryDelegationWindow(resource.StartTime, resource.EndTime); err != nil {
			return err
		}

		if hasDelegationOverlap(project.ID, *resource.StartTime, *resource.EndTime) {
			return fmt.Errorf("temporary delegation overlaps an existing delegation")
		}

		status := models.ProjectAdminDelegationStatus.APPROVED
		if time.Now().Before(*resource.StartTime) || !time.Now().Before(*resource.EndTime) {
			status = models.ProjectAdminDelegationStatus.APPROVED
		}

		delegation := &models.ProjectAdminDelegation{
			ID:         uuid.New(),
			ProjectID:  project.ID,
			FromAdmin:  project.AdminEmail,
			ToAdmin:    resource.Beneficiary,
			StartTime:  *resource.StartTime,
			EndTime:    *resource.EndTime,
			Status:     status,
			ApprovalID: approvalID,
			CreatedBy:  resource.Beneficiary,
			ApprovedBy: approvedBy,
		}

		if err := database.DB.Create(delegation).Error; err != nil {
			return fmt.Errorf("unable to create temporary project-admin delegation: %w", err)
		}

		return nil

	default:
		return fmt.Errorf("unsupported transfer_type. Supported values: permanent, temporary")
	}
}



// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/group.go

// Package controllers has all the handlers for user-ms
package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"os"
	"regexp"
	"strconv"
	"strings"

	gitclient "github.com/amartyaa/repas/common/git_client"
	"github.com/amartyaa/repas/common/utils"

	"slices"

	billingModel "github.com/amartyaa/repas/billing_microservice/billing/models"
	"github.com/amartyaa/repas/common/argocd"
	auditlogs "github.com/amartyaa/repas/common/audit_logs"
	commondb "github.com/amartyaa/repas/common/database"
	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/middleware"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/github"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateDefaultProject  creates the default project in database, every new user is part of default project
func CreateDefaultProject() {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Create Default Project API Invoked")
	var project models.Project
	res := database.DB.Where("name = ?", "Default").Find(&project)
	if res.Error != nil {
		logger.Errorln(res.Error)
		return
	}
	if res.RowsAffected == 0 {
		logger.Debugln("Default Project not found. Creating it now.")
		project.Name = "Default"
		project.IsDeleted = false
		database.DB.Create(&project)
		logEvent := &billingModel.AuditLog{
			ID:                project.ID,
			ResourceType:      "Project",
			Action:            "Create",
			ProjectID:         project.Name,
			ActionPerformedBy: "vcollab@vodafone.com",
			ResourceID:        project.ID.String(),
			ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
			Comments:          "",
		}

		gormDb, _ := database.DB.DB()
		pgDB := commondb.GetPGDB(gormDb)
		auditlogs.CreateEvent(&pgDB, logEvent)
		return
	}
	logger.Info("Default Project Created")
}

func convertMap(originalMap interface{}) map[string]float64 {
	convertedMap := map[string]float64{}
	for key, value := range originalMap.(map[string]interface{}) {
		convertedMap[key] = value.(float64)
	}

	return convertedMap
}

// CreateProject Controller creates project
func CreateProject(c *gin.Context) {
	//fetch the reserved keywords from the environment variable
	reservedKeywords := strings.Split(os.Getenv("RESERVED_KEYWORDS"), ",")
	var project models.Project
	reqID, _ := c.Get("request_id")
	userID := c.GetString("user_id")
	// userType, _ := c.Get("user_type")
	ip := c.ClientIP()
	permissions := middleware.FetchPermissions(userID)
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Inside Create Project")
	err := c.ShouldBind(&project)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err,
			"request_id": reqID,
		})
		return
	}

	// Check if user is DevOPs lead permission
	devopsLead := permissions["DevOps_Lead"].([]string)
	if len(devopsLead) == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}
	if project.Name == "" || !isValidRFC1123(project.Name) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Name Is required and should be valid RFC1123",
			"request_id": reqID,
		})
		return
	}
	// Check if project name is a reserved keyword
	for _, keyword := range reservedKeywords {
		if project.Name == keyword {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "The project name is a reserved keyword",
				"request_id": reqID,
			})
			return
		}
	}
	// Add user to project admin
	if isAvailable(project.ProjectAdmins, userID) == false {
		project.ProjectAdmins = append(project.ProjectAdmins, userID)

	}

	logger.Info("project :", project)
	// Check if project already exists
	res := database.DB.Where("name = ?", project.Name).Find(&project)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      res.Error,
			"request_id": reqID,
		})
		return
	}
	if res.RowsAffected > 0 && project.IsDeleted {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "Project with the same name was recently deleted. Please choose a different name.",
			"success":    false,
			"request_id": reqID,
		})
		return
	} else if res.RowsAffected > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "Project Already Exists",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	project.ID = uuid.New()
	// var data map[string]any
	_, err = onboardProject(project.Name, logger, project.AdminEmail, project)
	if err != nil {
		logger.Errorln("Error while onboarding project ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Error while onboarding project:- " + err.Error(),
			"request_id": reqID,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Error Occured While Creating Project",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	// Audit Log Event for Project Creation
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "Project",
		Action:            "Create",
		ProjectID:         project.Name,
		ActionPerformedBy: userID,
		ResourceID:        project.ID.String(),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          "",
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)

	projectResponse := models.ProjectResponseModel{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		ProjectMembers:  project.ProjectMembers,
		ProjectAdmins:   project.ProjectAdmins,
		AdminEmail:      project.AdminEmail,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		UserQuota:       project.UserQuota,
		CPUQuota:        project.CPUQuota,
		MemoryQuota:     project.MemoryQuota,
		StorageQuota:    project.StorageQuota,
		BuildQuota:      project.BuildQuota,
		Labels:          project.Labels,
		BU:              project.BU,
		CostCentre:      project.CostCentre,
		VDICPUQuota:     project.VDICPUQuota,
		VDIMemoryQuota:  project.VDIMemoryQuota,
		VDIStorageQuota: project.VDIStorageQuota,
		VDIBuildQuota:   project.VDIBuildQuota,
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Created Successfully",
		"body":       projectResponse,
		"request_id": reqID,
	})
}

// UpdateProject updates the product
func UpdateProject(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	userID, _ := c.Get("user_id")
	userEmail, _ := c.Get("user_email")
	ip := c.ClientIP()
	permissions := middleware.FetchPermissions(userID.(string))
	var project models.Project
	var incomingProject models.Project
	var user models.User
	var defaultProject models.Project
	projectID := c.Param("project_id")
	action := c.Param("action")
	logger.Info("action :", action)
	reqID, _ := c.Get("request_id")
	type ErrorMessage map[string]interface{}

	err := c.ShouldBind(&incomingProject)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err,
			"request_id": reqID,
		})
		return
	}

	// Load the project before authorizing against it — a zero-value project.Name
	// made the check below a no-op.
	if res := database.DB.Where("id = ?", projectID).Find(&project); res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"error":      "Project Does Not Exist.",
			"request_id": reqID,
		})
		return
	}
	// Apply Validation: deny when the caller is NOT a lead of this project
	// (previous condition was inverted and checked against an empty project.Name).
	if !isAvailable(permissions["DevOps_Lead"].([]string), project.Name) {
		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}
	logger.Info("Permissions :", permissions)

	if result := Contains([]string{models.ProjectAction.ADD, models.ProjectAction.REMOVE}, action); !result {
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"error":      "Action is not correct.Use 'add' or 'remove' as action ",
			"request_id": reqID,
		})
		return
	}

	logger.Info("Project Details :", incomingProject)

	var notExists []string
	var alreadyExists []string
	var invalidDomain []string
	var removedUserIDs []string
	if action == "remove" {
		// Get the admin email for the project
		var adminEmail string
		database.DB.Model(&project).Where("id = ?", projectID).Pluck("admin_email", &adminEmail)

		// Check if the user performing the action is the admin
		if userEmail != adminEmail {
			c.JSON(http.StatusForbidden, gin.H{
				"success":    false,
				"error":      "Only project admin can remove members",
				"request_id": reqID,
			})
			return
		}

		// Check if the admin is trying to remove himself
		if isAvailable(incomingProject.ProjectMembers, adminEmail) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success":    false,
				"error":      "Project admin cannot remove himself from the project",
				"request_id": reqID,
			})
			return
		}

		database.DB.Where("id = ?", projectID).Find(&project)
		for i := range incomingProject.ProjectMembers {
			database.DB.Where("email = ?", incomingProject.ProjectMembers[i]).Find(&user)
			for i, v := range project.ProjectMembers {
				if v == user.ID.String() {
					removedUserIDs = append(removedUserIDs, user.ID.String())
					project.ProjectMembers = slices.Delete(project.ProjectMembers, i, i+1)
				}
			}
			for i, v := range project.ProjectAdmins {
				if v == user.ID.String() {
					removedUserIDs = append(removedUserIDs, user.ID.String())
					project.ProjectAdmins = slices.Delete(project.ProjectAdmins, i, i+1)
				}
			}
		}
		// Audit Log Event for Project User Action
		logEvent := &billingModel.AuditLog{
			ID:                uuid.MustParse(reqID.(string)),
			ResourceType:      "Project",
			Action:            "User " + action,
			ProjectID:         project.Name,
			ActionPerformedBy: userID.(string),
			ResourceID:        project.ID.String(),
			ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
			Comments:          fmt.Sprintf("User %s %sed from project", removedUserIDs, action),
			IP:                ip,
		}
		gormDb, _ := database.DB.DB()
		pgDB := commondb.GetPGDB(gormDb)
		auditlogs.CreateEvent(&pgDB, logEvent)
	} else if action == "add" {
		var addedUserIDs []string
		for i := range incomingProject.ProjectMembers {
			email := incomingProject.ProjectMembers[i]
			var addUser models.User
			res := database.DB.Where("email = ?", email).Find(&addUser)
			if res.RowsAffected == 0 {
				notExists = append(notExists, email)
				continue
			}
			if isAvailable(project.ProjectMembers, addUser.ID.String()) || isAvailable(project.ProjectAdmins, addUser.ID.String()) {
				alreadyExists = append(alreadyExists, email)
				continue
			}
			project.ProjectMembers = append(project.ProjectMembers, addUser.ID.String())
			addedUserIDs = append(addedUserIDs, addUser.ID.String())
		}
		// Audit Log Event for Project User Action
		logEvent := &billingModel.AuditLog{
			ID:                uuid.MustParse(reqID.(string)),
			ResourceType:      "Project",
			Action:            "User " + action,
			ProjectID:         project.Name,
			ActionPerformedBy: userID.(string),
			ResourceID:        project.ID.String(),
			ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
			Comments:          fmt.Sprintf("User %s %sed to project", addedUserIDs, action),
			IP:                ip,
		}
		gormDb, _ := database.DB.DB()
		pgDB := commondb.GetPGDB(gormDb)
		auditlogs.CreateEvent(&pgDB, logEvent)
	}

	logger.Info("Action is completed", project)
	res := database.DB.Model(&project).Where("id = ?", projectID).Updates(project)
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      res.Error,
			"request_id": reqID,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Error Occured While Updating Project Details",
			"request_id": reqID,
		})
		return
	}
	resDefaultProject := database.DB.Model(&defaultProject).Where("name = ? AND is_deleted = ?", "Default", "false").Updates(defaultProject)
	if resDefaultProject.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      resDefaultProject.Error,
			"request_id": reqID,
		})
		return
	}
	if resDefaultProject.RowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Error Occured While Removing User Details",
			"request_id": reqID,
		})
		return
	}
	database.DB.Where("id = ?", projectID).Find(&project)

	// Create notifications for removed users
	for _, removedUserID := range removedUserIDs {
		notification := models.Notification{
			Description:    "User Removed From Project",
			ID:             uuid.New(),
			ResourceID:     project.ID,
			RequestFrom:    userID.(string),
			RequestTo:      removedUserID,
			EventName:      "USER_REMOVED_FROM_PROJECT",
			ResourceType:   "PROJECT",
			ResourceName:   "USER",
			ResourceStatus: "REMOVED",
			Status:         "UNREAD",
		}
		database.DB.Create(&notification)
	}

	var projectMembersEmail []string
	for i := range project.ProjectMembers {
		projectMembersEmail = append(projectMembersEmail, getUserEmailByID(uuid.MustParse(project.ProjectMembers[i])))
	}

	var projectAdminsEmail []string
	for i := range project.ProjectAdmins {
		projectAdminsEmail = append(projectAdminsEmail, getUserEmailByID(uuid.MustParse(project.ProjectAdmins[i])))
	}
	projectResponse := models.ProjectResponseModel{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		ProjectMembers:  projectMembersEmail,
		ProjectAdmins:   projectAdminsEmail,
		AdminEmail:      project.AdminEmail,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		UserQuota:       project.UserQuota,
		CPUQuota:        project.CPUQuota,
		MemoryQuota:     project.MemoryQuota,
		StorageQuota:    project.StorageQuota,
		BuildQuota:      project.BuildQuota,
		Labels:          project.Labels,
		BU:              project.BU,
		CostCentre:      project.CostCentre,
		VDICPUQuota:     project.VDICPUQuota,
		VDIMemoryQuota:  project.VDIMemoryQuota,
		VDIStorageQuota: project.VDIStorageQuota,
		VDIBuildQuota:   project.VDIBuildQuota,
	}
	errors := ErrorMessage{
		"not_exists":     notExists,
		"already_exists": alreadyExists,
		"invalid_domain": invalidDomain,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Details Updated Successfully",
		"body":       projectResponse,
		"request_id": reqID,
		"error":      errors,
	})
}

// UpdateProjectDescription allows project owner/project admin to edit description
func UpdateProjectDescription(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))

	projectID := c.Param("project_id")
	reqID, _ := c.Get("request_id")
	userEmail, _ := c.Get("user_email")

	logger.Info(
		"Update project description request received. project_id=", projectID,
		" request_id=", reqID,
		" user_email=", userEmail,
	)

	var project models.Project

	if !utils.IsValidUUID(projectID) {
		logger.Warn(
			"Invalid project ID received for description update. project_id=", projectID,
			" request_id=", reqID,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"error":      "Invalid Project ID, Project ID should be UUID",
			"request_id": reqID,
		})
		return
	}

	var request struct {
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(
			"Invalid request payload for project description update. project_id=", projectID,
			" request_id=", reqID,
			" error=", err,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"error":      "Invalid request payload. Please provide description.",
			"request_id": reqID,
		})
		return
	}

	res := database.DB.
		Where("id = ? AND is_deleted = ?", projectID, false).
		First(&project)

	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			logger.Warn(
				"Project not found for description update. project_id=", projectID,
				" request_id=", reqID,
			)

			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "Project Not Found",
				"request_id": reqID,
			})
			return
		}

		logger.Error(
			"Database error while fetching project for description update. project_id=", projectID,
			" request_id=", reqID,
			" error=", res.Error,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Unable to fetch project details. Please try again later.",
			"request_id": reqID,
		})
		return
	}

	if !IsEffectiveProjectAdmin(project.ID, userEmail.(string)) {
		logger.Warn(
			"Unauthorized project description update attempt. project_id=", projectID,
			" request_id=", reqID,
			" project_admin_email=", project.AdminEmail,
			" request_user_email=", userEmail,
		)

		c.JSON(http.StatusForbidden, gin.H{
			"success":    false,
			"error":      "Only project owner can update project description",
			"request_id": reqID,
		})
		return
	}

	updateRes := database.DB.
		Model(&project).
		Update("description", request.Description)

	if updateRes.Error != nil {
		logger.Error(
			"Database error while updating project description. project_id=", projectID,
			" request_id=", reqID,
			" error=", updateRes.Error,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Unable to update project description. Please try again later.",
			"request_id": reqID,
		})
		return
	}

	if updateRes.RowsAffected == 0 {
		logger.Warn(
			"Project description update completed with zero rows affected. project_id=", projectID,
			" request_id=", reqID,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Project description could not be updated.",
			"request_id": reqID,
		})
		return
	}

	project.Description = request.Description

	logger.Info(
		"Project description updated successfully. project_id=", projectID,
		" request_id=", reqID,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Project description updated successfully",
		"body": gin.H{
			"id":          project.ID,
			"description": project.Description,
		},
		"request_id": reqID,
	})
}

// Contains checks if array of string s contains string e
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// ExpandUserQuota increases the user quota in a project
func ExpandUserQuota(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	userID, _ := c.Get("user_id")
	permissions := middleware.FetchPermissions(userID.(string))
	var project models.Project
	projectID := c.Param("project_id")
	userType, _ := c.Get("user_type")
	reqID, _ := c.Get("request_id")
	err := c.ShouldBind(&project)
	if err != nil {
		logger.Error("ShouldBind error: ", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	logger.Info("Permissions :", permissions)

	if userType != "ADMIN" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}
	logger.Info("Project Details :", project)
	// Update the project's userquota in the database
	res := database.DB.Model(&project).Where("id = ?", projectID).Updates(map[string]interface{}{"userquota": project.UserQuota})
	if res.Error != nil {
		logger.Info("Error :", res.Error)
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Error Occured While Updating Project Details",
			"request_id": reqID,
		})
		return
	}
	_ = database.DB.Where("id = ?", projectID).Find(&project)

	projectResponse := models.ProjectResponseModel{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		ProjectMembers:  project.ProjectMembers,
		ProjectAdmins:   project.ProjectAdmins,
		AdminEmail:      project.AdminEmail,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		UserQuota:       project.UserQuota,
		CPUQuota:        project.CPUQuota,
		MemoryQuota:     project.MemoryQuota,
		StorageQuota:    project.StorageQuota,
		BuildQuota:      project.BuildQuota,
		Labels:          project.Labels,
		BU:              project.BU,
		CostCentre:      project.CostCentre,
		VDICPUQuota:     project.VDICPUQuota,
		VDIMemoryQuota:  project.VDIMemoryQuota,
		VDIStorageQuota: project.VDIStorageQuota,
		VDIBuildQuota:   project.VDIBuildQuota,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "User Quota Updated Successfully",
		"body":       projectResponse,
		"request_id": reqID,
	})
}

// GetProjectByID gets individual project
func GetProjectByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userType, _ := c.Get("user_type")
	permissions := middleware.FetchPermissions(userID.(string))
	var project models.Project
	var projectMembers []models.Member
	var projectAdmins []models.Member
	var memberUser models.User
	var adminUser models.User
	projectID := c.Param("project_id")
	reqID, _ := c.Get("request_id")
	if !utils.IsValidUUID(projectID) {
		c.JSON(
			http.StatusBadRequest, gin.H{
				"success":    false,
				"message":    "Invalid Project ID, Project ID should be UUID",
				"request_id": reqID,
			})
		return
	}

	if userType == "ADMIN" {
		res := database.DB.Where("id = ?", projectID).Find(&project)
		if res.Error != nil {
			c.JSON(500, gin.H{
				"success":    false,
				"error":      "Error Occured While Connecting To Database.",
				"request_id": reqID,
			})
			return
		}
		if res.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "You Are Not Part Of This Project.",
				"request_id": reqID,
			})
			return
		}
	} else {

		res := database.DB.Where("id = ? AND (name IN ? OR name IN ?)", projectID, permissions["DevOps"], permissions["DevOps_Lead"]).Find(&project)
		if res.Error != nil {
			c.JSON(500, gin.H{
				"success":    false,
				"error":      "Error Occured While Connecting To Database.",
				"request_id": reqID,
			})
			return
		}
		if res.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "You Are Not Part Of This Project.",
				"request_id": reqID,
			})
			return
		}
	}

	for j := 0; j < (len(project.ProjectMembers)); j++ {
		memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
		if memberRes.RowsAffected == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error Occured While Fetching User Details.",
			})
			return
		}
		member := models.Member{
			ID:    project.ProjectMembers[j],
			Email: memberUser.Email,
			Name:  memberUser.Name,
		}
		projectMembers = append(projectMembers, member)
		memberUser = models.User{}
	}

	for k := 0; k < (len(project.ProjectAdmins)); k++ {
		adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
		if adminRes.RowsAffected == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error Occured While Fetching User Details.",
			})
			return
		}
		admin := models.Member{
			ID:    project.ProjectAdmins[k],
			Email: adminUser.Email,
			Name:  adminUser.Name,
		}
		projectAdmins = append(projectAdmins, admin)
		adminUser = models.User{}
	}

	projectResponse := models.ProjectsResponseModel{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		ProjectMembers:  projectMembers,
		ProjectAdmins:   projectAdmins,
		AdminEmail:      project.AdminEmail,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		CPUQuota:        project.CPUQuota,
		MemoryQuota:     project.MemoryQuota,
		StorageQuota:    project.StorageQuota,
		BuildQuota:      project.BuildQuota,
		Labels:          project.Labels,
		BU:              project.BU,
		CostCentre:      project.CostCentre,
		VDICPUQuota:     project.VDICPUQuota,
		VDIMemoryQuota:  project.VDIMemoryQuota,
		VDIStorageQuota: project.VDIStorageQuota,
		VDIBuildQuota:   project.VDIBuildQuota,
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Details Fetched Successfully",
		"body":       projectResponse,
		"request_id": reqID,
	})
}

// GetProjects controllers gets list of projects
func GetProjects(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	userID, _ := c.Get("user_id")
	permissions := middleware.FetchPermissions(userID.(string))
	var projects []models.Project
	var memberUser models.User
	var adminUser models.User
	var projectResponse []models.ProjectsResponseModel
	userType, _ := c.Get("user_type")
	var res *gorm.DB
	reqID, _ := c.Get("request_id")
	orderByAcceptedValues := []string{"created_at"}
	sortAcceptedValues := []string{"ASC", "DESC"}
	var orderBy, sort string
	var limit int
	orderBy = c.Query("order_by")
	sort = c.Query("sort")
	limit, err := strconv.Atoi(string(c.Query("limit")))
	if err != nil {
		logger.Error("Error occured while converting limit :", err)
	}
	if sort == "" {
		sort = "DESC"
	}
	if orderBy == "" {
		orderBy = "created_at"
	}

	logger.Info("limit is set to :", limit)
	logger.Info("orderBy is set to :", orderBy)
	logger.Info("sort is set to :", sort)

	if !isAvailable(orderByAcceptedValues, orderBy) {
		c.JSON(400, gin.H{
			"success":    false,
			"error":      "Invalid request data. The 'order_by' field value is not valid!!!",
			"request_id": reqID,
		})
		return
	}

	if !isAvailable(sortAcceptedValues, sort) {
		c.JSON(400, gin.H{
			"success":    false,
			"error":      "Invalid request data. The 'sort' field value is not valid!!!",
			"request_id": reqID,
		})
		return
	}

	if userType == "ADMIN" {
		if limit != 0 {
			res = database.DB.
				Where("is_deleted = ?", false).
				Order(fmt.Sprintf("%s %s", orderBy, sort)).
				Limit(limit).
				Find(&projects)
		} else {
			res = database.DB.
				Where("is_deleted = ?", false).
				Order(fmt.Sprintf("%s %s", orderBy, sort)).
				Find(&projects)
			logger.Info("Admin made a get projects request")
		}
	} else {
		if limit != 0 {
			res = database.DB.Where("name IN ? OR name IN ?", permissions["DevOps"], permissions["DevOps_Lead"]).Order(fmt.Sprintf("%s %s", orderBy, sort)).Limit(limit).Find(&projects)
		} else {
			res = database.DB.Where("name IN ? OR name IN ?", permissions["DevOps"], permissions["DevOps_Lead"]).Order(fmt.Sprintf("%s %s", orderBy, sort)).Find(&projects)
		}
	}

	if res.Error != nil {
		c.JSON(500, gin.H{
			"success":    false,
			"error":      "Error Occured While Connecting To Database.",
			"request_id": reqID,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(200, gin.H{
			"success":    true,
			"message":    "No Projects Found",
			"body":       projectResponse,
			"request_id": reqID,
		})
		return
	}

	// logger.Info("Projects :", projects)
	for i := 0; i < (len(projects)); i++ {
		var projectMembers []models.Member
		var projectAdmins []models.Member

		for j := 0; j < (len(projects[i].ProjectMembers)); j++ {
			_ = database.DB.Where("id = ?", projects[i].ProjectMembers[j]).Find(&memberUser)

			member := models.Member{
				ID:    projects[i].ProjectMembers[j],
				Email: memberUser.Email,
				Name:  memberUser.Name,
			}
			projectMembers = append(projectMembers, member)
			memberUser = models.User{}
		}
		for k := 0; k < (len(projects[i].ProjectAdmins)); k++ {
			_ = database.DB.Where("id = ?", projects[i].ProjectAdmins[k]).Find(&adminUser)

			admin := models.Member{
				ID:    projects[i].ProjectAdmins[k],
				Email: adminUser.Email,
				Name:  adminUser.Name,
			}
			projectAdmins = append(projectAdmins, admin)
			adminUser = models.User{}
		}
		response := models.ProjectsResponseModel{
			ID:              projects[i].ID,
			Name:            projects[i].Name,
			Description:     projects[i].Description,
			ProjectMembers:  projectMembers,
			ProjectAdmins:   projectAdmins,
			AdminEmail:      projects[i].AdminEmail,
			CreatedAt:       projects[i].CreatedAt,
			UpdatedAt:       projects[i].UpdatedAt,
			CPUQuota:        projects[i].CPUQuota,
			MemoryQuota:     projects[i].MemoryQuota,
			StorageQuota:    projects[i].StorageQuota,
			BuildQuota:      projects[i].BuildQuota,
			Labels:          projects[i].Labels,
			BU:              projects[i].BU,
			CostCentre:      projects[i].CostCentre,
			VDICPUQuota:     projects[i].VDICPUQuota,
			VDIMemoryQuota:  projects[i].VDIMemoryQuota,
			VDIStorageQuota: projects[i].VDIStorageQuota,
			VDIBuildQuota:   projects[i].VDIBuildQuota,
		}
		projectResponse = append(projectResponse, response)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Details Fetched Successfully",
		"body":       projectResponse,
		"request_id": reqID,
	})
}

// DeleteProject deletes the project
func DeleteProject(c *gin.Context) {
	var project models.Project
	// var workspace models.Workspace
	// var template models.Template
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	projectID := c.Param("project_id")
	email, _ := c.Get("user_email")
	reqID, _ := c.Get("request_id")
	userID, _ := c.Get("user_id")
	userType, _ := c.Get("user_type")
	permissions := middleware.FetchPermissions(userID.(string))

	if userType == "ADMIN" {
		c.JSON(401, gin.H{
			"error":      "Product Administrators are restricted from deleting a project.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	//Find out the Project into the database by project_id
	res := database.DB.Where("id = ? AND (name IN ?)", projectID, permissions["DevOps_Lead"]).Find(&project)
	if res.Error != nil {
		c.JSON(500, gin.H{
			"success":    false,
			"error":      "Error Occured While Connecting To Database.",
			"request_id": reqID,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"success":    false,
			"error":      "Error Occured While Deleting Project.",
			"request_id": reqID,
		})
		return
	}
	if !IsEffectiveProjectAdmin(project.ID, email.(string)) {
		c.JSON(401, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}
	// check if project is DEFAULT - NO ONE can delete it
	if project.Name == "Default" {
		c.JSON(401, gin.H{
			"success":    false,
			"error":      "This Is A Default Project. It Can Not Be Deleted!",
			"request_id": reqID,
		})
		return
	}

	const attempts = 3
	const delay = 2 * time.Second

	isOffboarded, err := offboardProject(project.Name, logger)
	if err != nil {
		err = Retry(attempts, delay, func() error {
			var offboardErr error
			isOffboarded, offboardErr = offboardProject(project.Name, logger)
			if offboardErr != nil {
				return offboardErr
			}
			return nil
		})
		if err != nil {
			logger.Errorln("Error while offboarding project even after retries:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":    false,
				"error":      "Error while offboarding project:- " + err.Error(),
				"request_id": reqID,
			})
			return
		}
	}
	// Delete the project from the database
	type WorkspaceAndTemplate struct {
		ID            string
		ProjectID     string
		OperationName string
	}
	var wt []WorkspaceAndTemplate
	if isOffboarded {
		//delete workspace
		database.DB.Table("workspace").Select("*").Where("project_id = ? AND operation_name != ?", project.Name, "DELETION").Scan(&wt)
		for i := 0; i < len(wt); i++ {
			res = database.DB.Table("workspace").Where("id = ?", wt[i].ID).Update("operation_name", "DELETION")
			if res.Error != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"error":      "Error Occured While Connecting To Database.",
					"request_id": reqID,
				})
				return
			}
			if res.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"error":      "Error Occured While Deleting Workspace.",
					"request_id": reqID,
				})
				return
			}
		}

		//delete template
		wt = []WorkspaceAndTemplate{}
		database.DB.Table("template").Select("*").Where("project_id = ? AND operation_name != ?", project.Name, "DELETION").Scan(&wt)
		for i := 0; i < len(wt); i++ {
			res = database.DB.Table("template").Where("id = ?", wt[i].ID).Update("operation_name", "DELETION")
			if res.Error != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"error":      "Error Occured While Connecting To Database.",
					"request_id": reqID,
				})
				return
			}
			if res.RowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"error":      "Error Occured While Deleting Template.",
					"request_id": reqID,
				})
				return
			}
		}
		//delete users
		project.ProjectMembers = project.ProjectMembers[:0]
		project.ProjectAdmins = project.ProjectAdmins[:0]

		res := database.DB.Model(&project).Where("id = ?", projectID).Updates(project)
		if res.Error != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "Error Occured While Connecting To Database.",
				"request_id": reqID,
			})
			return
		}
		if res.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success":    false,
				"error":      "Error Occured While Deleting Users From Project.",
				"request_id": reqID,
			})
			return
		}

		//delete project
		res = database.DB.Model(&project).Where("id = ?", projectID).Update("is_deleted", true)
		if res.Error != nil {
			c.JSON(500, gin.H{
				"success":    false,
				"error":      "Error Occured While Connecting To Database.",
				"request_id": reqID,
			})
			return
		}
	} else {
		logger.Errorln("Project is not offboarded from Harbor")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Project is not offboarded from Harbor",
			"request_id": reqID,
		})
		return
	}

	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"error":      "Error Occured While Deleting Project.",
			"request_id": reqID,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Deleted Successfully",
		"id":         projectID,
		"request_id": reqID,
	})
}

func onboardProject(projectName string, logger *zap.SugaredLogger, email string, project models.Project) (map[string]any, error) {

	gitHost := os.Getenv("GIT_HOST")
	if gitHost == "" {
		logger.Error("Environment variable GIT_HOST is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_HOST is not set")
	}
	owner := os.Getenv("GIT_OWNER")
	if owner == "" {
		logger.Error("Environment variable GIT_OWNER is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_OWNER is not set")
	}
	repo := os.Getenv("GIT_REPO")
	if repo == "" {
		logger.Error("Environment variable GIT_REPO is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_REPO is not set")
	}
	branch := os.Getenv("GIT_BRANCH")
	if branch == "" {
		logger.Error("Environment variable GIT_BRANCH is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_BRANCH is not set")
	}
	dockerConfigJSON := os.Getenv("DOCKER_CONFIG_JSON")
	if dockerConfigJSON == "" {
		logger.Errorln("DOCKER_CONFIG_JSON not found in environment variables. Please set the HARBOR_DOCKER_CONFIG_JSON environment variable.")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_DOCKER_CONFIG_JSON is not set")
	}
	// Get GitHub client using centralized gitclient package
	httpClient, err := gitclient.GetGitClient()
	if err != nil {
		logger.Errorln("Error creating GitHub client: ", err)
		return nil, err
	}
	ghClient := gitclient.GetGitHubClient(httpClient)
	// STEP 0
	// email string conversion to valid RFC 1123 string

	email = strings.ToLower(email)
	email = strings.ReplaceAll(email, "@", "_")
	appEnvironment := os.Getenv("APP_ENV")

	managedBy := "openshift-gitops"
	if os.Getenv("PLATFORM") == "azure" {
		managedBy = "argocd"
	}
	// STEP 1
	// GITHUB DIRECTORY AND NAMESPACE MANIFEST CREATION
	//var project models.Project
	var buildQuota = int(project.BuildQuota + project.VDIBuildQuota)
	var nsCPU = int(project.MemoryQuota+project.VDIMemoryQuota) + buildQuota

	var nsMemory = int(project.MemoryQuota+project.VDIMemoryQuota) + buildQuota
	// TIP for the developer who will work here, use "spaces" instead of "tab" in case you want indentation.
	dockerCredentials := `apiVersion: v1
data:
  .dockerconfigjson: ` + dockerConfigJSON + `
kind: Secret
metadata:
  name: harbor-credentials
  namespace: ` + projectName + `
  labels:
    vcol_created_by: ` + utils.ConvertStringToRFC1123(email) + `
type: kubernetes.io/dockerconfigjson
`
	_ = fmt.Sprintf(`apiVersion: v1
kind: ResourceQuota
metadata:
  name: cpu-mem-quota
spec:
  hard:
    cpu: "%d"
    memory: "%dGi"`, nsCPU, nsMemory)
	ns := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  labels:
    project_created_by: "%s"
    project: "%s"
    part-of: "vcollab-%s"
    argocd.argoproj.io/managed-by: %s
  name: "%s"`, email, projectName, appEnvironment, managedBy, projectName)
	serviceAccount := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: ` + projectName + `-sa
  labels:
    part-of: vcollab-` + appEnvironment + `
    project_created_by: "` + email + `"
    project: "` + projectName + `"
  namespace: ` + projectName
	scc := `kind: SecurityContextConstraints
apiVersion: security.openshift.io/v1
metadata:
  namespace: ` + projectName + `
  name: scc-` + projectName + `
  labels:
    part-of: vcollab-` + appEnvironment + `
    project_created_by: "` + email + `"
    project: "` + projectName + `"
allowPrivilegedContainer: true
allowedCapabilities:
  - CHOWN
  - SETUID
  - SETGID
  - FOWNER
  - DAC_OVERRIDE
  - SETFCAP
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: RunAsAny
fsGroup:
  type: RunAsAny
supplementalGroups:
  type: RunAsAny
users:
  - system:serviceaccount:` + projectName + `:` + projectName + `-sa`
	nad := `apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    description: Linux Bridge with CNV Network - VLAN 860
  name: br-cnv-nad-` + projectName + `
  labels:
    part-of: vcollab-` + appEnvironment + `
  namespace:  ` + projectName + `
spec:
  config: >-
    {"name":"br-cnv-nad-` + projectName + `","type":"cnv-bridge","cniVersion":"0.4.0","promiscMode": true,"bridge":"bridge860","macspoofchk":true,"ipam":{}}`
	dvCloneCR := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    part-of: vcollab-` + appEnvironment + `
    project_created_by: "` + email + `"
    project: "` + projectName + `"
  name: dv-clone-cr-` + projectName + `
rules:
- apiGroups: ["cdi.kubevirt.io"]
  resources: ["datavolumes/source"]
  verbs: ["*"]`
	dvCloneRB := `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dv-clone-rb-` + projectName + `
  namespace: default
  labels:
    part-of: vcollab-` + appEnvironment + `
subjects:
- kind: ServiceAccount
  name: ` + projectName + `-sa
  namespace: ` + projectName + `
- kind: ServiceAccount
  name: default
  namespace: ` + projectName + `
roleRef:
  kind: ClusterRole
  name: dv-clone-cr-` + projectName + `
  apiGroup: rbac.authorization.k8s.io`
	netpol := `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-vcollab-ns
  namespace: ` + projectName + `
spec:
  podSelector: {} 
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector: 
            matchExpressions:
              - key: kubernetes.io/metadata.name 
                operator: In
                values: [dev, qa, prod]
  egress:
    - {}`

	entries := []github.TreeEntry{
		{
			Path:    github.String("tenants/" + projectName + "/namespace.yaml"),
			Type:    github.String("blob"),
			Content: github.String(ns),
			Mode:    github.String("100644"),
		},
		{
			Path:    github.String("tenants/" + projectName + "/docker-credentials.yaml"),
			Type:    github.String("blob"),
			Content: github.String(dockerCredentials),
			Mode:    github.String("100644"),
		},
		{
			Path:    github.String("tenants/" + projectName + "/serviceaccount.yaml"),
			Type:    github.String("blob"),
			Content: github.String(serviceAccount),
			Mode:    github.String("100644"),
		},
		{
			Path:    github.String("tenants/" + projectName + "/networkpolicy.yaml"),
			Type:    github.String("blob"),
			Content: github.String(netpol),
			Mode:    github.String("100644"),
		},
	}
	if os.Getenv("PLATFORM") == "openshift" {
		entries = append(entries,
			github.TreeEntry{
				Path:    github.String("tenants/" + projectName + "/scc.yaml"),
				Type:    github.String("blob"),
				Content: github.String(scc),
				Mode:    github.String("100644"),
			},
			github.TreeEntry{
				Path:    github.String("tenants/" + projectName + "/nad.yaml"),
				Type:    github.String("blob"),
				Content: github.String(nad),
				Mode:    github.String("100644"),
			},
			github.TreeEntry{
				Path:    github.String("tenants/" + projectName + "/clusterrole.yaml"),
				Type:    github.String("blob"),
				Content: github.String(dvCloneCR),
				Mode:    github.String("100644"),
			},
			github.TreeEntry{
				Path:    github.String("tenants/" + projectName + "/rolebinding.yaml"),
				Type:    github.String("blob"),
				Content: github.String(dvCloneRB),
				Mode:    github.String("100644"),
			},
		)
	}
	ghClient.CommitContent(entries, "Created Namespace & SA for "+projectName)

	// STEP 2
	// ARGOCD LOGIN, APP CREATION & SYNC
	argoURL := os.Getenv("ARGOCD_URL")
	if argoURL == "" {
		logger.Error("Environment variable ARGOCD_URL is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable ARGOCD_URL is not set")
	}
	login := argocd.Client(argoURL)
	if login == "" {
		logger.Errorln("Error logging in ArgoCD")
	}

	create := argocd.Create(branch, owner, projectName, repo, gitHost, argoURL)
	if create != nil {
		logger.Errorln("Error craeting app in ArgoCD")
	}

	sync := argocd.Sync(projectName, argoURL)
	if sync != nil {
		logger.Errorln("Error syncing", projectName)
	}

	httpclient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// STEP 3
	// HARBOR PROJECT CREATION

	harborURL := os.Getenv("HARBOR_URL")
	if harborURL == "" {
		logger.Errorln("Environment variable HARBOR_URL is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_URL is not set")
	}
	harborUser := os.Getenv("HARBOR_USER")
	if harborUser == "" {
		logger.Errorln("Environment variable HARBOR_USER is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_USER is not set")
	}
	harborPass := os.Getenv("HARBOR_PASS")
	if harborPass == "" {
		logger.Errorln("Environment variable HARBOR_PASS is not set")
		return nil, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_PASS is not set")
	}
	harborProject := map[string]interface{}{
		"project_name": projectName,
		"public":       true,
		"metadata": map[string]interface{}{
			"public":                        "false",
			"enable_content_trust":          "false",
			"enable_content_trust_cosign":   "false",
			"enable_content_trust_notation": "false",
			"prevent_vul":                   "false",
			"severity":                      "none",
			"auto_scan":                     "true",
			"reuse_sys_cve_whitelist":       "false",
		}}
	harborProjectJSON, _ := json.Marshal(harborProject)
	req, _ := http.NewRequest("POST", harborURL+"/api/v2.0/projects", bytes.NewBuffer(harborProjectJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(harborUser, harborPass)
	resp, err := httpclient.Do(req)
	if err != nil {
		logger.Errorln("Error while creating project in harbor  ", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		logger.Errorln("Error while creating project in harbor  ", resp.StatusCode)
		err = fmt.Errorf("Error while creating project in harbor: %d", resp.StatusCode)
		return nil, err
	}

	// After successful project creation
	immutabilityRule := map[string]interface{}{
		"disabled": false,
		"action":   "immutable",
		"template": "immutable", // Added: Harbor often requires this template definition
		"tag_selectors": []map[string]interface{}{
			{
				"kind":       "doublestar",
				"decoration": "matches",
				"pattern":    "**", // Changed to ** to robustly match all tags
			},
		},
		"scope_selectors": map[string][]map[string]interface{}{
			"repository": {
				{
					"kind":       "doublestar",
					"decoration": "matches",
					"pattern":    "**", // Matches all repos in the project
				},
			},
		},
	}

	immutabilityJSON, _ := json.Marshal(immutabilityRule)
	reqRule, _ := http.NewRequest("POST", harborURL+"/api/v2.0/projects/"+projectName+"/immutabletagrules", bytes.NewBuffer(immutabilityJSON))
	reqRule.Header.Set("Content-Type", "application/json")
	reqRule.SetBasicAuth(harborUser, harborPass)

	respRule, err := httpclient.Do(reqRule)
	if err != nil {
		logger.Errorln("Error while adding immutability rule in harbor ", err)
		return nil, err
	}
	defer respRule.Body.Close()

	if respRule.StatusCode != 200 && respRule.StatusCode != 201 {
		logger.Errorln("Error while adding immutability rule in harbor ", respRule.StatusCode)
		err = fmt.Errorf("Error while adding immutability rule in harbor: %d", respRule.StatusCode)
		return nil, err
	}

	logger.Infoln("Immutability rule applied successfully to project", projectName)
	return nil, nil
}

func offboardProject(projectName string, logger *zap.SugaredLogger) (bool, error) {
	httpclient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	owner := os.Getenv("GIT_OWNER")
	if owner == "" {
		logger.Error("Environment variable GIT_OWNER is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_HOST is not set")
	}
	repo := os.Getenv("GIT_REPO")
	if repo == "" {
		logger.Error("Environment variable GIT_REPO is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_REPO is not set")
	}
	branch := os.Getenv("GIT_BRANCH")
	if branch == "" {
		logger.Error("Environment variable GIT_BRANCH is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable GIT_BRANCH is not set")
	}
	argoURL := os.Getenv("ARGOCD_URL")
	if argoURL == "" {
		logger.Errorln("Environment variable ARGOCD_URL is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable ARGOCD_URL is not set")
	}

	argoUser := os.Getenv("ARGOCD_USER")
	if argoUser == "" {
		logger.Errorln("Environment variable ARGOCD_USER is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable ARGOCD_USER is not set")
	}
	argoPass := os.Getenv("ARGOCD_PASS")
	if argoPass == "" {
		logger.Errorln("Environment variable ARGOCD_PASS is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable ARGOCD_PASS is not set")
	}
	// STEP 1
	// DELETE HARBOR PROJECT
	harborURL := os.Getenv("HARBOR_URL")
	if harborURL == "" {
		logger.Errorln("Environment variable HARBOR_URL is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_URL is not set")
	}
	harborUser := os.Getenv("HARBOR_USER")
	if harborUser == "" {
		logger.Errorln("Environment variable HARBOR_USER is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_USER is not set")
	}
	harborPass := os.Getenv("HARBOR_PASS")
	if harborPass == "" {
		logger.Errorln("Environment variable HARBOR_PASS is not set")
		return false, errors.New(http.StatusText(http.StatusInternalServerError) + ": environment variable HARBOR_PASS is not set")
	}
	harborProject := map[string]interface{}{}
	harborProjectJSON, _ := json.Marshal(harborProject)

	req, _ := http.NewRequest("GET", harborURL+"/api/v2.0/projects?name="+projectName, bytes.NewBuffer(harborProjectJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(harborUser, harborPass)
	resp, err := httpclient.Do(req)
	if err != nil {
		logger.Errorln("Error while getting project in harbor  ", err)
		return false, err
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	var projects []map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &projects); err != nil {
		logger.Errorln("Error while unmarshalling project in harbor  ", err)
		return false, err
	}
	var ID float64
	for _, project := range projects {
		if project["name"] == projectName {
			logger.Infoln("project found in harbor  ", project["project_id"])
			ID = project["project_id"].(float64)
		}
	}

	req, _ = http.NewRequest("DELETE", harborURL+"/api/v2.0/projects/"+fmt.Sprint(ID), bytes.NewBuffer(harborProjectJSON))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(harborUser, harborPass)
	resp, err = httpclient.Do(req)
	if err != nil {
		logger.Errorln("Error while deleting project in harbor  ", err)
		return false, err
	}
	defer resp.Body.Close()

	// STEP 2
	// DELETE ARGOCD APP

	creds := map[string]interface{}{
		"username": argoUser,
		"password": argoPass,
	}
	argocdToken := ""
	credsJSON, _ := json.Marshal(creds)
	req, _ = http.NewRequest("POST", argoURL+"/api/v1/session", bytes.NewBuffer(credsJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err = httpclient.Do(req)
	if err != nil {
		logger.Errorln("Error while logging in argocd  ", err)
		return false, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	argocdTokenInterface, ok := result["token"]
	if !ok {
		return false, fmt.Errorf("error while logging into argo %v", result)
	}
	argocdToken = argocdTokenInterface.(string)

	// argoProj := os.Getenv("ARGOCD_PROJECT")
	// if argoProj == "" {
	// 	logger.Debugln("ARGOCD_PROJECT not found in environment variables, using default project vcollab-platform-manager")
	// 	argoProj = "vcollab-platform-manager"
	// }

	appJSON, _ := json.Marshal(map[string]interface{}{})

	req, _ = http.NewRequest("DELETE", argoURL+"/api/v1/applications/"+projectName+"?cascade=true&propagationPolicy=foreground", bytes.NewBuffer(appJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+argocdToken)

	resp, err = httpclient.Do(req)
	if err != nil {
		logger.Errorln("Error while creating application in argocd  ", err)
		return false, err
	}
	defer resp.Body.Close()
	logger.Debugln(resp.StatusCode)

	// STEP 3
	// DELETE GITHUB DIRECTORY AND NAMESPACE MANIFEST

	httpClient, err := gitclient.GetGitClient()
	if err != nil {
		logger.Errorln("Error creating GitHub client:", err)
		return false, err
	}
	ghClient := gitclient.GetGitHubClient(httpClient)

	ctx := context.Background()
	folderPath := "tenants/" + projectName

	// Gather all file paths within the namespace folder.
	filePaths, err := ghClient.GatherFilePaths(ctx, owner, repo, folderPath, branch)
	if err != nil {
		logger.Errorln("Error gathering file paths for deletion:", err)
		return false, err
	}

	// Delete all files using the centralized DeleteFiles function.
	err = ghClient.DeleteFiles(filePaths, "Deleted entire namespace for "+projectName, branch, owner, repo)
	if err != nil {
		logger.Errorln("Error deleting namespace files:", err)
		return false, err
	}
	logger.Infoln("Successfully deleted namespace folder and its contents:", folderPath)

	logger.Infoln("Successfully deleted folder and its contents:", folderPath)
	return true, nil
}

// deleteFolderContents retrieves the contents of folderPath using the GitHub Contents API.
// It recursively deletes files (and enters subdirectories) calling itself.
func deleteFolderContents(ctx context.Context, ghClient *gitclient.VFGitHubClient, owner, repo, branch, folderPath string, logger *zap.SugaredLogger) error {
	// GetContents returns (fileContent, directoryContent, response, err).
	fileContent, dirContents, _, err := ghClient.Repositories.GetContents(ctx, owner, repo, folderPath, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			logger.Debugf("Folder %s not found, nothing to delete", folderPath)
			return nil
		}
		return err
	}

	// If dirContents is non-nil, the path is a directory.
	if dirContents != nil {
		for _, item := range dirContents {
			if item == nil {
				continue
			}
			if item.GetType() == "dir" {
				// Recursively delete files within the subdirectory.
				if err := deleteFolderContents(ctx, ghClient, owner, repo, branch, item.GetPath(), logger); err != nil {
					return err
				}
			} else if item.GetType() == "file" {
				options := &github.RepositoryContentFileOptions{
					Message: github.String("Deleted " + item.GetPath() + " during offboarding"),
					SHA:     github.String(item.GetSHA()),
					Branch:  github.String(branch),
				}
				_, _, err := ghClient.Repositories.DeleteFile(ctx, owner, repo, item.GetPath(), options)
				if err != nil {
					logger.Errorf("Error deleting file %s: %v", item.GetPath(), err)
					return err
				}
				logger.Debugf("Deleted file: %s", item.GetPath())
			}
		}
	} else if fileContent != nil {
		// If fileContent is non-nil, then folderPath actually points to a file.
		options := &github.RepositoryContentFileOptions{
			Message: github.String("Deleted file " + fileContent.GetPath() + " during offboarding"),
			SHA:     github.String(fileContent.GetSHA()),
			Branch:  github.String(branch),
		}
		_, _, err := ghClient.Repositories.DeleteFile(ctx, owner, repo, fileContent.GetPath(), options)
		if err != nil {
			logger.Errorf("Error deleting file %s: %v", fileContent.GetPath(), err)
			return err
		}
		logger.Debugf("Deleted file: %s", fileContent.GetPath())
	}
	return nil
}

func isValidRFC1123(textInput string) bool {

	// Check length
	if len(textInput) < 1 || len(textInput) > 63 {
		return false
	}

	// Check characters
	matched, _ := regexp.MatchString("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", textInput)
	return matched
}

// GetProjectByName takes project name as input and returns project details.
//
// Access is scoped to anyone logged in as this function will be used in "Find Project" which can be done by anyone who has logged in on V-Collab
func GetProjectByName(c *gin.Context) {
	var project models.Project
	var projectMembers []models.Member
	var projectAdmins []models.Member
	var memberUser models.User
	var adminUser models.User
	projectName := c.Param("project_name")
	reqID, _ := c.Get("request_id")
	userID, _ := c.Get("user_id")
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info(userID, " did request to get project ", projectName, " by name under request id ", reqID)

	var res *gorm.DB
	res = database.DB.Where("name = ?", projectName).Find(&project)
	if res.RowsAffected == 0 || project.ID.String() == "" || project.ID.String() == "00000000-0000-0000-0000-000000000000" {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"error":      "Project Not Found",
			"request_id": reqID,
		})
		return
	}
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      res.Error,
			"request_id": reqID,
		})
		return
	}
	for j := 0; j < (len(project.ProjectMembers)); j++ {
		memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
		if memberRes.RowsAffected == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error Occured While Fetching User Details.",
			})
			return
		}
		member := models.Member{
			ID:    project.ProjectMembers[j],
			Email: memberUser.Email,
			Name:  memberUser.Name,
		}
		projectMembers = append(projectMembers, member)
		memberUser = models.User{}
	}

	for k := 0; k < (len(project.ProjectAdmins)); k++ {
		adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
		if adminRes.RowsAffected == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Error Occured While Fetching User Details.",
			})
			return
		}
		admin := models.Member{
			ID:    project.ProjectAdmins[k],
			Email: adminUser.Email,
			Name:  adminUser.Name,
		}
		projectAdmins = append(projectAdmins, admin)
		adminUser = models.User{}
	}
	projectResponse := models.ProjectsResponseModel{
		ID:              project.ID,
		Name:            project.Name,
		Description:     project.Description,
		ProjectMembers:  projectMembers,
		ProjectAdmins:   projectAdmins,
		AdminEmail:      project.AdminEmail,
		CreatedAt:       project.CreatedAt,
		UpdatedAt:       project.UpdatedAt,
		CPUQuota:        project.CPUQuota,
		MemoryQuota:     project.MemoryQuota,
		StorageQuota:    project.StorageQuota,
		BuildQuota:      project.BuildQuota,
		Labels:          project.Labels,
		BU:              project.BU,
		CostCentre:      project.CostCentre,
		VDICPUQuota:     project.VDICPUQuota,
		VDIMemoryQuota:  project.VDIMemoryQuota,
		VDIStorageQuota: project.VDIStorageQuota,
		VDIBuildQuota:   project.VDIBuildQuota,
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Project Fetched Successfully",
		"body":       projectResponse,
		"request_id": reqID,
	})

}

// Retry executes the given function `fn` up to `attempts` times, waiting `delay` duration between attempts if an error occurs.
func Retry(attempts int, delay time.Duration, fn func() error) error {
	for i := 0; i < attempts; i++ {
		err := fn()
		if err == nil {
			return nil // Success
		}
		//log.Printf("Attempt %d failed: %v. Retrying in %v...", i+1, err, delay)
		time.Sleep(delay)
	}
	return fn() // Final attempt return value
}




// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/project_stats.go

package controllers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/middleware"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
)

// GetProjectStats sends the stats of particular project
func GetProjectStats(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Project Status API Invoked")
	reqID, _ := c.Get("request_id")
	projectID := c.Param("project_id")
	userID := c.GetString("user_id")
	userType, _ := c.Get("user_type")

	type quotaMap struct {
		Allocated float64 `json:"allocated"`
		Consumed  float64 `json:"consumed"`
	}

	type resourcesQuota struct {
		IDE struct {
			CPUQuota     quotaMap `json:"cpu_quota"`
			MemoryQuota  quotaMap `json:"memory_quota"`
			StorageQuota quotaMap `json:"storage_quota"`
		} `json:"ide"`
		CNV struct {
			CPUQuota     quotaMap `json:"cpu_quota"`
			MemoryQuota  quotaMap `json:"memory_quota"`
			StorageQuota quotaMap `json:"storage_quota"`
		} `json:"cnv"`
	}

	type overview struct {
		Projects struct {
			Count       int    `json:"count"`
			Limit       int    `json:"limit"`
			Description string `json:"description"`
		} `json:"projects"`
		Workspaces map[string]int `json:"workspaces"`
		Templates  map[string]int `json:"templates"`
		Builds     map[string]int `json:"builds"`
		Users      map[string]int `json:"users"`
	}

	type stats struct {
		Overview       overview       `json:"overview"`
		ResourcesQuota resourcesQuota `json:"resources_quota"`
	}

	var project models.Project
	var ws []struct {
		ID       string
		Type     string
		TfConfig json.RawMessage `gorm:"column:tfconfig"`
	}
	var resultTemplates []struct {
		ID string
	}
	type Build struct {
		ID string
	}
	var resultBuilds []Build
	var builds []Build
	var cnvBuilds []Build
	var data []string
	var ideCpuquota, ideMemoryquota, ideStoragequota float64
	var cnvCpuquota, cnvMemoryquota, cnvStoragequota float64

	database.DB.Where("name = ? AND is_deleted=?", projectID, false).Find(&project)
	permissions := middleware.FetchPermissions(userID)
	devopsLead := permissions["DevOps_Lead"].([]string)
	devops := permissions["DevOps"].([]string)

	// docs/pentest/user_microservice-findings.md #14: without this check, the
	// project record (description, quota limits) below was returned to any
	// authenticated caller regardless of membership — only the consumption
	// numbers in the branches below were gated.
	if userType != "ADMIN" && !isAvailable(devopsLead, projectID) && !isAvailable(devops, projectID) {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"error":      "You Are Not Part Of This Project.",
			"request_id": reqID,
		})
		return
	}

	if userType == "ADMIN" || isAvailable(devopsLead, projectID) {
		database.DB.Table("workspace").Select("*").Where("project_id = ? AND NOT (operation_name = 'DELETION' AND operation_status = 'SUCCESS')", projectID).Scan(&ws)
		for _, w := range ws {
			var tfConfig map[string]interface{}
			json.Unmarshal(w.TfConfig, &tfConfig)
			if w.Type == "IDE" || w.Type == "Linux_VDI" {
				ideCpuquota += tfConfig["cpu"].(float64)
				ideMemoryquota += tfConfig["memory"].(float64)
				ideStoragequota += tfConfig["drive"].(float64)
			} else if w.Type == "Linux_CNV" || w.Type == "Windows_CNV" || w.Type == "Azure_Linux_VM" || w.Type == "Azure_Windows_VM" || w.Type == "OCI_VM_Linux" || w.Type == "OCI_VM_Windows" {
				cnvCpuquota += tfConfig["cpu"].(float64)
				cnvMemoryquota += tfConfig["memory"].(float64)
				cnvStoragequota += tfConfig["drive"].(float64)
			}
		}

		database.DB.Table("template").Select("*").Where("project_id = ? AND operation_name != ?", projectID, "DELETION").Scan(&resultTemplates)
		database.DB.Where("name = ? AND is_deleted=?", projectID, false).Order("created_at asc").Find(&project)
		for _, adminID := range project.ProjectAdmins {
			var user models.User
			database.DB.Where("id = ? AND is_deleted=?", adminID, false).Find(&user)
			data = append(data, user.Email)
		}
		for _, memberID := range project.ProjectMembers {
			var user models.User
			database.DB.Where("id = ? AND is_deleted=?", memberID, false).Find(&user)
			data = append(data, user.Email)
		}
		if os.Getenv("PLATFORM") == "azure" {
			database.DB.Table("azurevmbuild").Select("*").Where("project_id = ?", projectID).Scan(&cnvBuilds)
		} else if os.Getenv("PLATFORM") == "oci" {
			var drccBuilds []Build
			var ociBuilds []Build
			// Scan cnvbuilds
			database.DB.Table("cnvbuilds").Select("*").Where("project_id = ?", projectID).Scan(&ociBuilds)
			// Scan drcccnvbuilds
			database.DB.Table("drcccnvbuilds").Select("*").Where("project_id = ?", projectID).Scan(&drccBuilds)
			cnvBuilds = append(ociBuilds, drccBuilds...)
		} else {
			database.DB.Table("cnvbuilds").Select("*").Where("project_id = ?", projectID).Scan(&cnvBuilds)
		}
		database.DB.Table("builds").Select("*").Where("project_id = ?", projectID).Scan(&builds)
		resultBuilds = append(builds, cnvBuilds...)
	}

	if isAvailable(devops, projectID) {
		database.DB.Table("workspace").Select("*").Where("created_by = ? AND project_id = ? AND NOT (operation_name = 'DELETION' AND operation_status = 'SUCCESS')", c.GetString("user_email"), projectID).Scan(&ws)
		for _, w := range ws {
			var tfConfig map[string]interface{}
			json.Unmarshal(w.TfConfig, &tfConfig)
			if w.Type == "IDE" || w.Type == "Linux_VDI" {
				ideCpuquota += tfConfig["cpu"].(float64)
				ideMemoryquota += tfConfig["memory"].(float64)
				ideStoragequota += tfConfig["drive"].(float64)
			} else if w.Type == "Linux_CNV" || w.Type == "Windows_CNV" || w.Type == "Azure_Linux_VM" || w.Type == "Azure_Windows_VM" || w.Type == "OCI_VM_Linux" || w.Type == "OCI_VM_Windows" {
				cnvCpuquota += tfConfig["cpu"].(float64)
				cnvMemoryquota += tfConfig["memory"].(float64)
				cnvStoragequota += tfConfig["drive"].(float64)
			}
		}
		database.DB.Table("template").Select("*").Where("created_by = ? AND project_id = ? AND operation_name != ?", c.GetString("user_email"), projectID, "DELETION").Scan(&resultTemplates)
		database.DB.Where("admin_email =? AND name = ? AND is_deleted=?", c.GetString("user_email"), projectID, false).Order("created_at asc").Find(&project)
		for _, adminID := range project.ProjectAdmins {
			var user models.User
			database.DB.Where("id = ? AND is_deleted=?", adminID, false).Find(&user)
			data = append(data, user.Email)
		}
		for _, memberID := range project.ProjectMembers {
			var user models.User
			database.DB.Where("id = ? AND is_deleted=?", memberID, false).Find(&user)
			data = append(data, user.Email)
		}
		var builds []Build
		var cnvBuilds []Build
		if os.Getenv("PLATFORM") == "azure" {
			database.DB.Table("azurevmbuild").Select("*").Where("created_by = ? AND project_id = ?", c.GetString("user_email"), projectID).Scan(&cnvBuilds)
		} else if os.Getenv("PLATFORM") == "oci" {
			var drccBuilds []Build
			var ociBuilds []Build
			// Scan cnvbuilds
			database.DB.Table("cnvbuilds").Select("*").Where("created_by = ? AND project_id = ?", c.GetString("user_email"), projectID).Scan(&cnvBuilds)
			// Scan drcccnvbuilds
			database.DB.Table("drcccnvbuilds").Select("*").Where("created_by = ? AND project_id = ?", c.GetString("user_email"), projectID).Scan(&drccBuilds)
			cnvBuilds = append(ociBuilds, drccBuilds...)
		} else {
			database.DB.Table("cnvbuilds").Select("*").Where("created_by = ? AND project_id = ?", c.GetString("user_email"), projectID).Scan(&cnvBuilds)
		}
		database.DB.Table("builds").Select("*").Where("created_by = ? AND project_id = ?", c.GetString("user_email"), projectID).Scan(&builds)
		resultBuilds = append(builds, cnvBuilds...)
	}

	stat := stats{
		Overview: overview{
			Projects: struct {
				Count       int    `json:"count"`
				Limit       int    `json:"limit"`
				Description string `json:"description"`
			}{
				Count:       1,
				Limit:       5,
				Description: project.Description,
			},
			Workspaces: map[string]int{
				"count": len(ws),
				"limit": -1,
			},
			Templates: map[string]int{
				"count": len(resultTemplates),
				"limit": -1,
			},
			Builds: map[string]int{
				"count": len(resultBuilds),
				"limit": project.BuildQuota + project.VDIBuildQuota,
			},
			Users: map[string]int{
				"count": len(data),
				"limit": project.UserQuota,
			},
		},
		ResourcesQuota: resourcesQuota{
			IDE: struct {
				CPUQuota     quotaMap `json:"cpu_quota"`
				MemoryQuota  quotaMap `json:"memory_quota"`
				StorageQuota quotaMap `json:"storage_quota"`
			}{
				CPUQuota: quotaMap{
					Allocated: project.CPUQuota,
					Consumed:  ideCpuquota,
				},
				MemoryQuota: quotaMap{
					Allocated: project.MemoryQuota,
					Consumed:  ideMemoryquota,
				},
				StorageQuota: quotaMap{
					Allocated: project.StorageQuota,
					Consumed:  ideStoragequota,
				},
			},
			CNV: struct {
				CPUQuota     quotaMap `json:"cpu_quota"`
				MemoryQuota  quotaMap `json:"memory_quota"`
				StorageQuota quotaMap `json:"storage_quota"`
			}{
				CPUQuota: quotaMap{
					Allocated: project.VDICPUQuota,
					Consumed:  cnvCpuquota,
				},
				MemoryQuota: quotaMap{
					Allocated: project.VDIMemoryQuota,
					Consumed:  cnvMemoryquota,
				},
				StorageQuota: quotaMap{
					Allocated: project.VDIStorageQuota,
					Consumed:  cnvStoragequota,
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Successfully Project Count Fetched.",
		"body":       stat,
		"request_id": reqID,
	})
}



// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/user_test.go

package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildUserUpdates_NonAdminCannotEscalate is the regression test for the
// ADMIN privilege-escalation bug (docs/pentest/user_microservice-findings.md #1,
// VERIFIED live): a non-admin sending {"type":"ADMIN"} must never see that field
// make it into the update set.
func TestBuildUserUpdates_NonAdminCannotEscalate(t *testing.T) {
	updates := buildUserUpdates(false, "PASSWORD", UpdateUserRequest{
		Name: "New Name",
		Type: "ADMIN",
	})

	assert.Equal(t, "New Name", updates["name"], "self-edit of name should still work")
	_, hasType := updates["type"]
	assert.False(t, hasType, "non-admin must never be able to set type")
	_, hasStatus := updates["status"]
	assert.False(t, hasStatus, "non-admin must never be able to set status")
}

func TestBuildUserUpdates_NonAdminCannotSetStatus(t *testing.T) {
	updates := buildUserUpdates(false, "PASSWORD", UpdateUserRequest{Status: "INACTIVE"})
	_, hasStatus := updates["status"]
	assert.False(t, hasStatus, "non-admin must never be able to set status")
}

func TestBuildUserUpdates_AzureUserProfileLocked(t *testing.T) {
	updates := buildUserUpdates(false, "AZURE", UpdateUserRequest{Name: "Attempted Change"})
	assert.Empty(t, updates, "Azure-login users' profile fields must be locked — Azure is the source of truth")
}

func TestBuildUserUpdates_AzureAdminCanStillChangeRole(t *testing.T) {
	// An admin editing an Azure-login user's role/status is allowed — only the
	// name (profile detail owned by Azure) is locked.
	updates := buildUserUpdates(true, "AZURE", UpdateUserRequest{Name: "Ignored", Status: "INACTIVE", Type: "ADMIN"})
	assert.Equal(t, "INACTIVE", updates["status"])
	assert.Equal(t, "ADMIN", updates["type"])
	_, hasName := updates["name"]
	assert.False(t, hasName, "name stays locked for Azure users even when an admin is editing")
}

func TestBuildUserUpdates_AdminCanChangeRole(t *testing.T) {
	updates := buildUserUpdates(true, "PASSWORD", UpdateUserRequest{Type: "ADMIN", Status: "ACTIVE"})
	assert.Equal(t, "ADMIN", updates["type"])
	assert.Equal(t, "ACTIVE", updates["status"])
}

func TestBuildUserUpdates_NoFieldsProvided(t *testing.T) {
	updates := buildUserUpdates(false, "PASSWORD", UpdateUserRequest{})
	assert.Empty(t, updates)
}

// TestDummyPasswordHash_IsValidForTimingSafety is the regression test for the
// login-enumeration fix (findings #4): the dummy hash used when no user is
// found must be a real bcrypt hash so the comparison costs the same as a real
// one, not a fast-failing malformed string.
func TestDummyPasswordHash_IsValidForTimingSafety(t *testing.T) {
	assert.NotEmpty(t, dummyPasswordHash)
	// A real bcrypt comparison against it must run (and fail, since the
	// password is wrong) rather than error out on a malformed hash.
	assert.False(t, _CheckPasswordHash("some-random-guess", dummyPasswordHash))
}



// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/user.go
package controllers

import (
	"net/http"
	"os"
	"time"

	billingModel "github.com/amartyaa/repas/billing_microservice/billing/models"
	auditlogs "github.com/amartyaa/repas/common/audit_logs"
	commondb "github.com/amartyaa/repas/common/database"
	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/helpers"
	"github.com/amartyaa/repas/user_microservice/user/middleware"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateUser function is used to create a new user
func CreateUser(c *gin.Context) {
	var user models.User
	var auth models.Auth
	reqID, _ := c.Get("request_id")
	ip := c.ClientIP()

	if res := helpers.CheckUserType(c, "ADMIN"); res != true {
		c.JSON(401, gin.H{
			"error":      "Unauthorized to access this resource",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	err := c.ShouldBind(&user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	if !helpers.IsStrongPassword(user.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Password must be 6-20 characters and include an uppercase letter, a lowercase letter, a digit, and a special character.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	if user.Type != "USER" {
		user.Type = "USER"
	}
	authRes := database.DB.Where("user_email = ?", user.Email).Find(&auth)
	if authRes.RowsAffected == 1 {
		c.JSON(409, gin.H{
			"error":      "User Already Exists.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	// tmp_usr_id := strings.Split(uuid.New().String(), "-")
	// user.ID = tmp_usr_id[len(tmp_usr_id)-1]

	hashedPWD, err := HashPassword(user.Password)
	user.Password = hashedPWD

	res := database.DB.Create(&user)

	if res.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Error While Creating User.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	CreateUserRecord(c, (user.ID).String(), user.Email, hashedPWD, "PASSWORD")
	AddToDefaultGroup(c, (user.ID).String())
	// Audit Log Event for User Creation
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "User",
		Action:            "Create",
		ProjectID:         "Default",
		ActionPerformedBy: user.Email,
		ResourceID:        (user.ID).String(),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          "",
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"id":         user.ID,
		"message":    "User Created Successfully",
		"request_id": reqID,
	})
}

// GetUser function is used to get users by user ID(mandatory parameter)
func GetUser(c *gin.Context) {
	var user models.User
	userID := c.Param("user_id")
	reqID, _ := c.Get("request_id")
	if res := helpers.MatchUserIDToUID(c, userID); res != true {
		c.JSON(401, gin.H{
			"error":      "Unauthorized to access this resource",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	res := database.DB.Where("id = ?", userID).Find(&user)
	if res.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   res.Error,
			"success": false,
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"success":    false,
			"error":      "User Does Not Exists.",
			"request_id": reqID,
		})
		return
	}
	platform := "k8s"
	if os.Getenv("PLATFORM") != "" {
		platform = os.Getenv("PLATFORM")
	}

	userResponse := models.UserResponseModel{
		Name:        user.Name,
		Email:       user.Email,
		ID:          user.ID,
		Type:        user.Type,
		Status:      user.Status,
		Permissions: middleware.FetchPermissions((user.ID).String()),
		Platform:    platform,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "User Details Fetched Successfully",
		"body":       userResponse,
		"request_id": reqID,
	})
}

// UpdateUserRequest is the request body for UpdateUser. Only the fields a caller is
// allowed to change are bound here — never bind models.User directly for an update,
// or a client can set type/status/is_deleted via mass assignment.
type UpdateUserRequest struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// buildUserUpdates decides which columns a PUT /users/{id} caller is allowed to
// change, given their role and the target user's login type. Kept as a pure
// function (no DB/gin dependency) so the authorization decision itself — the
// part that matters for docs/pentest/user_microservice-findings.md #1 — is
// directly unit-testable.
func buildUserUpdates(isAdmin bool, targetLoginType string, req UpdateUserRequest) map[string]interface{} {
	updates := map[string]interface{}{}
	if targetLoginType != "AZURE" && req.Name != "" {
		updates["name"] = req.Name
	}
	if isAdmin {
		if req.Status != "" {
			updates["status"] = req.Status
		}
		if req.Type != "" {
			updates["type"] = req.Type
		}
	}
	return updates
}

// UpdateUser function is used to update the user details.
//
// Editable fields depend on the caller's role and the target user's login type:
//   - name: password-login users only (self or admin) — locked for Azure-login users,
//     since Azure is the source of truth for profile details.
//   - email: never editable here (identity key, tied to auth/Azure).
//   - status, type: ADMIN only.
//   - is_deleted: never via this endpoint.
func UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	var user models.User
	userID := c.Param("user_id")
	reqID, _ := c.Get("request_id")
	ip := c.ClientIP()
	err := c.ShouldBind(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err,
		})
		return
	}
	isAdmin := helpers.CheckUserType(c, "ADMIN")
	if !isAdmin && helpers.MatchUserIDToUID(c, userID) != true {
		c.JSON(401, gin.H{
			"error":      "Unauthorized to access this resource",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	var auth models.Auth
	authRes := database.DB.Where("user_id = ?", userID).Find(&auth)
	if authRes.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"error":      "User Does Not Exists.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	updates := buildUserUpdates(isAdmin, auth.LoginType, req)
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"error":      "No editable fields provided for this user",
			"request_id": reqID,
		})
		return
	}

	res := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
	if res.RowsAffected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Error Occured While Updating User Detail",
		})
		return
	}
	res = database.DB.Where("id = ?", userID).Find(&user)

	platform := "k8s"
	if os.Getenv("PLATFORM") != "" {
		platform = os.Getenv("PLATFORM")
	}

	userResponse := models.UserResponseModel{
		Name:        user.Name,
		Email:       user.Email,
		ID:          user.ID,
		Type:        user.Type,
		Status:      user.Status,
		Permissions: middleware.FetchPermissions((user.ID).String()),
		Platform:    platform,
	}
	// Audit Log Event for User Update
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "User",
		Action:            "Update",
		ProjectID:         "Default",
		ActionPerformedBy: user.Email,
		ResourceID:        (user.ID).String(),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          "",
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "User Updated Successfully",
		"body":       userResponse,
		"request_id": reqID,
	})
}

// DeleteUser function is used to delete a user
func DeleteUser(c *gin.Context) {
	var user models.User
	var auth models.Auth
	userID := c.Param("user_id")
	reqID, _ := c.Get("request_id")
	ip := c.ClientIP()
	if res := helpers.CheckUserType(c, "ADMIN"); res != true {
		if res := helpers.MatchUserIDToUID(c, userID); res != true {

			c.JSON(401, gin.H{
				"error":      "Unauthorized to access this resource",
				"success":    false,
				"request_id": reqID,
			})
			return
		}
	}
	authRes := database.DB.Where("id = ?", userID).Find(&auth)
	if authRes.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"error":      "User Does Not exists.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	// Load the user record so the audit log below has a real actor/resource
	// instead of an empty email and zero UUID (docs/pentest/user_microservice-findings.md #18).
	database.DB.Where("id = ?", userID).Find(&user)

	authRes = database.DB.Where("id = ?", userID).Delete(&auth)
	if authRes.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"error":      "User Does Not exists.",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	res := database.DB.Model(&user).Where("id = ?", userID).UpdateColumn("is_deleted", true)
	if res.Error != nil {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "Error Occured While Connecting To Database.",
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success":    false,
			"error":      "Error Occured While Deleting User.",
			"request_id": reqID,
		})
		return
	}

	// Audit Log Event for User Deletion
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "User",
		Action:            "Delete",
		ProjectID:         "Default",
		ActionPerformedBy: user.Email,
		ResourceID:        (user.ID).String(),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          "",
		IP:                ip,
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "User Deleted Successfully",
		"id":         userID,
		"request_id": reqID,
	})
}

// ResetPassword function is used to reset the password
func ResetPassword(c *gin.Context) {
	var auth models.Auth
	//Adding comment so git glitch is fixed here
	rp := models.ResetPass{} //Adding comment so git glitch is fixed here
	userID, _ := c.Get("user_id")
	reqID, _ := c.Get("request_id")
	userEmail, _ := c.Get("user_email")

	err := c.ShouldBind(&rp)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Data should not be empty",
			"success":    false,
			"request_id": reqID,
		})
		return
	}
	authRes := database.DB.Where("user_id = ? AND user_email= ?", userID, userEmail).Find(&auth)
	if authRes.RowsAffected == 0 {
		c.JSON(404, gin.H{
			"error":      "User does not exists",
			"success":    false,
			"request_id": reqID,
		})
		return
	}

	//Compare current password and  existing hash password from db
	match := _CheckPasswordHash(rp.CurrentPassword, auth.UserPassword)
	if !match {
		c.JSON(http.StatusExpectationFailed, gin.H{
			"success": false,
			"error":   "Current password does not matched!!!.",
		})
		return
	}
	if !helpers.IsStrongPassword(rp.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":    false,
			"error":      "Password must be 6-20 characters and include an uppercase letter, a lowercase letter, a digit, and a special character.",
			"request_id": reqID,
		})
		return
	}
	//
	hashedPWD, _ := HashPassword(rp.NewPassword)
	res := database.DB.Model(&auth).Where("user_id = ?", userID).UpdateColumn("user_password", hashedPWD)
	if res.Error != nil {
		c.JSON(417, gin.H{
			"success": false,
			"error":   "Error Occured While Updating  User Password.",
		})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusExpectationFailed, gin.H{
			"success":    false,
			"error":      "Error Occured While Updating Auths.",
			"request_id": reqID,
		})
		return
	}
	// Audit Log Event for Password Reset. Uses the authenticated caller's own
	// identity from context — this endpoint always acts on the caller's own
	// account (see the auth lookup above) — instead of an unpopulated models.User
	// (docs/pentest/user_microservice-findings.md #18).
	logEvent := &billingModel.AuditLog{
		ID:                uuid.MustParse(reqID.(string)),
		ResourceType:      "User",
		Action:            "Password Reset",
		ProjectID:         "Default",
		ActionPerformedBy: userEmail.(string),
		ResourceID:        userID.(string),
		ActionDoneAt:      time.Now().Format("2006-01-02 15:04:05.000000-07"),
		Comments:          "",
	}
	gormDb, _ := database.DB.DB()
	pgDB := commondb.GetPGDB(gormDb)
	auditlogs.CreateEvent(&pgDB, logEvent)
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Password Reset Successfully",
		"request_id": reqID,
	})
}

// GetUserStats function is used to get the user stats
func GetUserStats(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	logger.Info("Project Status API Invoked")
	reqID, _ := c.Get("request_id")
	userType := c.GetString("user_type")
	userID := c.Param("user_id")
	var user models.User
	logger.Info("user_id:-----", userID)
	if userType != "ADMIN" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"error":      "Unauthorized To Access This Resource",
			"request_id": reqID,
		})
		return
	}
	type templateMap map[string]interface{}
	type workspaceMap map[string]interface{}
	type buildMap map[string]interface{}
	var data []string
	var templateObject map[string]interface{}
	var workspaceObject map[string]interface{}
	var buildObject map[string]interface{}
	type stats struct {
		Workspaces workspaceMap `json:"workspaces"`
		Templates  templateMap  `json:"templates"`
		Builds     templateMap  `json:"builds"`
	}

	type Results struct {
		ProjectID string
		ID        string
	}
	var result []Results

	//workspace
	database.DB.Where("id = ?", userID).Find(&user)
	logger.Info(" Requested user email:-----", user.Email)
	database.DB.Table("workspace").Select("*").Where("created_by = ?", user.Email).Scan(&result)
	for i := 0; i < len(result); i++ {
		data = append(data, result[i].ID)
	}
	workspaceObject = workspaceMap{
		"workspaces": data,
		"count":      len(data),
	}
	data = []string{}
	result = []Results{}
	//template
	database.DB.Table("template").Select("*").Where("created_by = ? ", user.Email).Scan(&result)
	for i := 0; i < len(result); i++ {
		data = append(data, result[i].ID)
	}
	templateObject = templateMap{
		"templates": data,
		"count":     len(data),
	}
	data = []string{}
	result = []Results{}
	//build
	database.DB.Table("builds").Select("*").Where("created_by = ?", user.Email).Scan(&result)
	for i := 0; i < len(result); i++ {
		data = append(data, result[i].ID)
	}
	buildObject = buildMap{
		"builds": data,
		"count":  len(data),
	}

	stat := stats{
		Templates:  templateObject,
		Workspaces: workspaceObject,
		Builds:     buildObject,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Successfully Project Count Fetched.",
		"body":       stat,
		"request_id": reqID,
	})
}



// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/controllers/users.go

package controllers

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/common/utils"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/middleware"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type usersResponseModel struct {
	Name        string                 `json:"name"`
	Email       string                 `json:"email"`
	ID          uuid.UUID              `json:"id"`
	JoinedOn    time.Time              `json:"joined_on"`
	Type        string                 `json:"type"`
	Permissions map[string]interface{} `json:"permissions"`
}

// ListUsers function is used to get all users
func ListUsers(c *gin.Context) {
	// var user models.User
	var users []models.User
	var project models.Project
	var res *gorm.DB
	var limit int
	var err error
	var memberUser models.User
	var adminUser models.User
	requestID, _ := c.Get("request_id")
	loggerInstance := logger.CreateSugaredLogger(requestID.(string), logger.CreateCustomLogger(false, false, false))
	projectID := c.Query("project_id")
	projectName := c.Query("project_name")
	sortOrder := c.Query("sort")
	orderBy := c.Query("order_by")
	userEmail, _ := c.Get("user_email")
	userType, _ := c.Get("user_type")
	orderByAcceptedValues := []string{"created_at"}
	sortAcceptedValues := []string{"ASC", "DESC"}
	userID, _ := c.Get("user_id")
	var userResponse usersResponseModel
	var listUserResponse []usersResponseModel

	apiResp := gin.H{
		"success":    false,
		"error":      "",
		"body":       listUserResponse,
		"request_id": requestID,
	}

	permissions := middleware.FetchPermissions(userID.(string))
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	if orderBy == "" {
		orderBy = "created_at"
	}
	limit, err = strconv.Atoi(c.Query("limit"))
	if err != nil || limit < 0 {
		limit = 0
	}
	loggerInstance.Debugf("%s of usertype %s has sent get users request id %s with projectID %v limit %d orderby %s sort %s", userEmail, userType, requestID, projectID, limit, orderBy, sortOrder)
	if !utils.IsStringPartOfStringArray(orderByAcceptedValues, orderBy) {
		apiResp["error"] = "Invalid request data. The 'order_by' field value is not valid!!!"
		c.JSON(400, apiResp)
		return
	}
	if !utils.IsStringPartOfStringArray(sortAcceptedValues, sortOrder) {
		apiResp["error"] = "Invalid request data. The 'sort' field value is not valid!!!"
		c.JSON(400, apiResp)
		return
	}
	if projectID == "" && projectName == "" && userType != "ADMIN" { // case 1: when user is not PaaS Admin and has not sent project id/project name, send nothing
		apiResp["error"] = "Invalid request data. The 'project_id' or 'project_name' field is required for non Admins."
		c.JSON(400, apiResp)
		return
	} else if (projectID != "" || projectName != "") && userType != "ADMIN" { // case 2: when user is not PaaS Admin and has sent project id or project name, check if project admin, if yes send list of all users in project.

		// check if valid project id or valid project name
		if projectID != "" {
			userEmailKeyVakPair := make(map[string]bool) //checks if email already exists in listUserResponse
			if !utils.IsValidUUID(projectID) {
				apiResp["error"] = "Invalid request data. The 'project_id' field value is not valid!!!"
				c.JSON(400, apiResp)
				return
			}
			//Check if DevOps Lead of that Project
			res = database.DB.Where("id = ? AND (name IN ?)", projectID, permissions["DevOps_Lead"]).Find(&project)
			if res.RowsAffected == 0 || project.ID.String() == "" || project.ID.String() == "00000000-0000-0000-0000-000000000000" || project.Name == "" {
				// If Not DevOpsLead, send an empty response
				apiResp["message"] = "Project ID " + projectID + " not found or You don't have required permissions on Project."
				c.JSON(200, apiResp)
				return
			}
			if res.Error != nil {
				apiResp["error"] = res.Error
				c.JSON(http.StatusInternalServerError, apiResp)
				return
			}
			for k := 0; k < (len(project.ProjectAdmins)); k++ {
				adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
				if adminRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[adminUser.Email]; !exists { //checks if email already exists in listUserResponse
						admin := usersResponseModel{
							ID:       adminUser.ID,
							Email:    adminUser.Email,
							Name:     adminUser.Name,
							JoinedOn: adminUser.CreatedAt,
							Type:     "DevOps_Lead",
						}
						listUserResponse = append(listUserResponse, admin)
					}
					userEmailKeyVakPair[adminUser.Email] = true
					adminUser = models.User{}
				}
			}
			for j := 0; j < (len(project.ProjectMembers)); j++ {
				memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
				if memberRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[memberUser.Email]; !exists { //checks if email already exists in listUserResponse
						member := usersResponseModel{
							ID:       memberUser.ID,
							Email:    memberUser.Email,
							Name:     memberUser.Name,
							JoinedOn: memberUser.CreatedAt,
							Type:     "DevOps",
						}
						listUserResponse = append(listUserResponse, member)
					}
					userEmailKeyVakPair[memberUser.Email] = true
					memberUser = models.User{}
				}
			}
			userEmailKeyVakPair = map[string]bool{}
		} else if projectName != "" {
			userEmailKeyVakPair := make(map[string]bool) //checks if email already exists in listUserResponse
			res = database.DB.Where("name = ? AND (name IN ?)", projectName, permissions["DevOps_Lead"]).Find(&project)
			if res.RowsAffected == 0 || project.ID.String() == "" || project.ID.String() == "00000000-0000-0000-0000-000000000000" || project.Name == "" {
				apiResp["message"] = "Project Name " + projectName + " not found or You don't have required permissions on Project."
				c.JSON(200, apiResp)
				return
			}
			if res.Error != nil {
				apiResp["error"] = res.Error
				c.JSON(http.StatusInternalServerError, apiResp)
				return
			}
			for k := 0; k < (len(project.ProjectAdmins)); k++ {
				adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
				if adminRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[adminUser.Email]; !exists { //checks if email already exists in listUserResponse
						admin := usersResponseModel{
							ID:       adminUser.ID,
							Email:    adminUser.Email,
							Name:     adminUser.Name,
							JoinedOn: adminUser.CreatedAt,
							Type:     "DevOps_Lead",
						}
						listUserResponse = append(listUserResponse, admin)
					}
					userEmailKeyVakPair[adminUser.Email] = true
					adminUser = models.User{}
				}
			}
			for j := 0; j < (len(project.ProjectMembers)); j++ {
				memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
				if memberRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[memberUser.Email]; !exists { //checks if email already exists in listUserResponse
						member := usersResponseModel{
							ID:       memberUser.ID,
							Email:    memberUser.Email,
							Name:     memberUser.Name,
							JoinedOn: memberUser.CreatedAt,
							Type:     "DevOps",
						}
						listUserResponse = append(listUserResponse, member)
					}
					userEmailKeyVakPair[memberUser.Email] = true
					memberUser = models.User{}
				}
			}
			userEmailKeyVakPair = map[string]bool{}
		}
	} else if projectID == "" && projectName == "" && userType == "ADMIN" { // case 3: when user is PaaS Admin and has not sent project id/projectName, send list of all users
		if limit != 0 {
			database.DB.Order(fmt.Sprintf("%s %s", orderBy, sortOrder)).Limit(limit).Find(&users)
		} else {
			database.DB.Order(fmt.Sprintf("%s %s", orderBy, sortOrder)).Find(&users)
		}
		for i := 0; i < len(users); i++ {
			userResponse = usersResponseModel{
				Name:        users[i].Name,
				Email:       users[i].Email,
				ID:          users[i].ID,
				JoinedOn:    users[i].CreatedAt,
				Type:        users[i].Type,
				Permissions: middleware.FetchPermissions((users[i].ID).String()),
			}
			listUserResponse = append(listUserResponse, userResponse)
		}
		apiResp["success"] = true
		apiResp["body"] = listUserResponse
		c.JSON(200, apiResp)
		return
	} else if (projectID != "" || projectName != "") && userType == "ADMIN" { // case 4: when user is PaaS Admin and has sent project id or project name, send list of all users in project.
		if projectID != "" {
			userEmailKeyVakPair := make(map[string]bool) //checks if email already exists in listUserResponse
			if !utils.IsValidUUID(projectID) {
				apiResp["error"] = "Invalid request data. The 'project_id' field value is not valid!!!"
				c.JSON(400, apiResp)
				return
			}
			//Check if DevOps Lead of that Project
			res = database.DB.Where("id = ?", projectID).Find(&project)
			if res.RowsAffected == 0 || project.ID.String() == "" || project.ID.String() == "00000000-0000-0000-0000-000000000000" || project.Name == "" {
				// If Not DevOpsLead, send an empty response
				apiResp["error"] = "Project ID " + projectID + " not found."
				c.JSON(404, apiResp)
				return
			}
			if res.Error != nil {
				apiResp["error"] = res.Error
				c.JSON(http.StatusInternalServerError, apiResp)
				return
			}
			for k := 0; k < (len(project.ProjectAdmins)); k++ {
				adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
				if adminRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[adminUser.Email]; !exists { //checks if email already exists in listUserResponse
						admin := usersResponseModel{
							ID:       adminUser.ID,
							Email:    adminUser.Email,
							Name:     adminUser.Name,
							JoinedOn: adminUser.CreatedAt,
							Type:     "DevOps_Lead",
						}
						listUserResponse = append(listUserResponse, admin)
					}
					userEmailKeyVakPair[adminUser.Email] = true
					adminUser = models.User{}
				}
			}
			for j := 0; j < (len(project.ProjectMembers)); j++ {
				memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
				if memberRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[memberUser.Email]; !exists { //checks if email already exists in listUserResponse
						member := usersResponseModel{
							ID:       memberUser.ID,
							Email:    memberUser.Email,
							Name:     memberUser.Name,
							JoinedOn: memberUser.CreatedAt,
							Type:     "DevOps",
						}
						listUserResponse = append(listUserResponse, member)
					}
					userEmailKeyVakPair[memberUser.Email] = true
					memberUser = models.User{}
				}
			}
			userEmailKeyVakPair = map[string]bool{}
		} else if projectName != "" {
			userEmailKeyVakPair := make(map[string]bool) //checks if email already exists in listUserResponse
			res = database.DB.Where("name = ?", projectName).Find(&project)
			if res.RowsAffected == 0 || project.ID.String() == "" || project.ID.String() == "00000000-0000-0000-0000-000000000000" || project.Name == "" {
				apiResp["error"] = "Project Name " + projectName + " not found."
				c.JSON(404, apiResp)
				return
			}
			if res.Error != nil {
				apiResp["error"] = res.Error
				c.JSON(http.StatusInternalServerError, apiResp)
				return
			}
			for k := 0; k < (len(project.ProjectAdmins)); k++ {
				adminRes := database.DB.Where("id = ?", project.ProjectAdmins[k]).Find(&adminUser)
				if adminRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[adminUser.Email]; !exists { //checks if email already exists in listUserResponse
						admin := usersResponseModel{
							ID:       adminUser.ID,
							Email:    adminUser.Email,
							Name:     adminUser.Name,
							JoinedOn: adminUser.CreatedAt,
							Type:     "DevOps_Lead",
						}
						listUserResponse = append(listUserResponse, admin)
					}
					userEmailKeyVakPair[adminUser.Email] = true
					adminUser = models.User{}
				}
			}
			for j := 0; j < (len(project.ProjectMembers)); j++ {
				memberRes := database.DB.Where("id = ?", project.ProjectMembers[j]).Find(&memberUser)
				if memberRes.RowsAffected != 0 {
					if _, exists := userEmailKeyVakPair[memberUser.Email]; !exists { //checks if email already exists in listUserResponse
						member := usersResponseModel{
							ID:       memberUser.ID,
							Email:    memberUser.Email,
							Name:     memberUser.Name,
							JoinedOn: memberUser.CreatedAt,
							Type:     "DevOps",
						}
						listUserResponse = append(listUserResponse, member)
					}
					userEmailKeyVakPair[memberUser.Email] = true
					memberUser = models.User{}
				}
			}
			userEmailKeyVakPair = map[string]bool{}
		}
	}

	// Sorting
	slices.SortStableFunc(listUserResponse, func(i, j usersResponseModel) int {
		if sortOrder == "DESC" {
			if i.JoinedOn.Before(j.JoinedOn) {
				return 1
			} else if i.JoinedOn.After(j.JoinedOn) {
				return -1
			}
			return 0
		}
		if i.JoinedOn.Before(j.JoinedOn) {
			return -1
		} else if i.JoinedOn.After(j.JoinedOn) {
			return 1
		}
		return 0
	})

	// Limiting and offsetting
	var limitedListUserResponse []usersResponseModel
	responseLength := len(listUserResponse)
	if limit != 0 && limit < responseLength { // to prevent array overflow
		limitedListUserResponse = listUserResponse[0:limit]
	} else {
		limitedListUserResponse = listUserResponse
	}
	apiResp["success"] = true
	apiResp["body"] = limitedListUserResponse
	c.JSON(200, apiResp)
}




// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/middleware/auth.go

// Package middleware contains all three middlewares for USER-MS
package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/amartyaa/repas/common/logger"
	"github.com/amartyaa/repas/user_microservice/user/database"
	"github.com/amartyaa/repas/user_microservice/user/helpers"
	"github.com/amartyaa/repas/user_microservice/user/models"
	"github.com/gin-gonic/gin"
)

// Authenticate function authenticates incoming requuests
func Authenticate(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	requestID, _ := c.Get("request_id")
	clientToken := c.Request.Header.Get("Authorization")
	if clientToken == "" {
		logger.Infoln("No Authorization Header found for request ID", requestID)
		c.SetCookie("token", "", -1, "/", "", true, true)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No Authorization Header provided.", "request_id": requestID})
		c.Abort()
		return
	}

	// Validate that token in present in auth able or not
	// if !ValidateTokenInAuthTable(clientToken) {
	// 	c.JSON(401, gin.H{
	// 		"message": "Unauthorized To Access This Resource.",
	// 		"success": false,
	// 	})
	// 	c.Abort()
	// 	return
	// }

	// identifying the authentication provider
	var auth models.Auth
	authRes := database.DB.Where("token= ?", clientToken).Find(&auth)
	logger.Debugf("Auth response :%+v", auth)
	if authRes.RowsAffected == 0 {
		c.SetCookie("token", "", -1, "/", "", true, true)
		c.JSON(401, gin.H{
			"success":    false,
			"message":    "You are already logged out from the session!!! Kindly login again.",
			"request_id": requestID,
		})
		logger.Infoln("Invalid Token. Token not found in DB for request id", requestID, "Token", clientToken)
		c.Abort()
		return
	}
	disableAzureTokenVerification := false

	if strings.ToUpper(os.Getenv("DISABLE_AZURE_VERIFICATION")) == "TRUE" {
		disableAzureTokenVerification = true
	}
	logger.Infoln("Running With Azure Verification ", !disableAzureTokenVerification)

	if auth.LoginType == "AZURE" {
		logger.Infoln("Token is generated by azure AD for auth", auth.UserID, "request ID", requestID)
		var success bool
		var msg string
		if !disableAzureTokenVerification {
			success, _, _, _, msg = helpers.ValidateTokenForAzure(clientToken)
		} else {
			success = true
			msg = "Azure Token Validation is Disabled"
		}
		if success {
			logger.Infof("claims: %+v", msg)

			var user models.User
			res := database.DB.Where("id= ?", auth.UserID).Find(&user)
			if res.RowsAffected != 0 && user.IsDeleted {
				c.SetCookie("token", "", -1, "/", "", true, true)
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"message": "User Not Found.",
				})
				logger.Errorf("Azure user not found or deleted. %s", user.Email)
				c.Abort()
				return
			}
			c.Set("user_type", user.Type)
			c.Set("user_id", auth.UserID)
			c.Set("user_email", auth.UserEmail)
			c.Next()
		}
		if !success {
			c.JSON(http.StatusUnauthorized, gin.H{"success": success, "msg": msg})
			c.Abort()
			return
		}
	}

	if auth.LoginType == "PASSWORD" {
		//validate that user isdeleted is true or not in user table
		claims, err := helpers.ValidateToken(c, clientToken)
		logger.Debugf("Claims for : %+v , request id : %s", claims, requestID)
		if claims == nil {
			logger.Errorf("Token Is Expired, request id : %s", requestID)
			c.SetCookie("token", "", -1, "/", "", true, true)
			c.Abort()
			return
		}
		if err != "" {
			c.SetCookie("token", "", -1, "/", "", true, true)
			logger.Debugln("Message while Validating Token %s", err)
			c.JSON(http.StatusUnauthorized, gin.H{"message": err})
			c.Abort()
			return
		}
		var user models.User
		res := database.DB.Where("id= ?", claims.ID).Find(&user)
		if res.RowsAffected != 0 && user.IsDeleted {
			c.SetCookie("token", "", -1, "/", "", true, true)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "User Not Found.",
			})
			logger.Errorf("The User Is Deleted. %s", user.Email)
			c.Abort()
			return
		}

		c.Set("user_type", claims.UserType)
		c.Set("user_id", claims.ID)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

// Verify verifies token in all incoming requuests
func Verify(c *gin.Context) {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	requestID, _ := c.Get("request_id")
	clientToken := c.Request.Header.Get("Authorization")
	disableAzureTokenVerification := false

	if strings.ToUpper(os.Getenv("DISABLE_AZURE_VERIFICATION")) == "TRUE" {
		disableAzureTokenVerification = true
	}
	logger.Infoln("Running With Azure Verification ", !disableAzureTokenVerification)
	if clientToken == "" {
		logger.Infoln("No Authorization Header found for request ID", requestID)
		c.JSON(401, gin.H{"error": "No Authorization Header provided.", "request_id": requestID})
		c.Abort()
		return
	}

	// Validate that token in present in auth able or not
	// if !ValidateTokenInAuthTable(clientToken) {
	// 	c.JSON(401, gin.H{
	// 		"message": "Unauthorized To Access This Resource.",
	// 		"success": false,
	// 	})
	// 	c.Abort()
	// 	return
	// }

	// identifying the authentication provider
	var auth models.Auth
	authRes := database.DB.Where("token= ?", clientToken).Find(&auth)
	if authRes.RowsAffected == 0 {
		c.SetCookie("token", "", -1, "/", "", true, true)
		c.JSON(401, gin.H{
			"success":    false,
			"message":    "Invalid Token",
			"request_id": requestID,
		})
		logger.Infoln("Invalid Token. Token not found in DB for request id", requestID, "Token", clientToken)
		c.Abort()
		return
	}
	logger.Debugf("Verify API of request id %s invoked by %s with login type %s", requestID, auth.UserEmail, auth.LoginType)
	if auth.LoginType == "AZURE" {
		var success bool
		var msg string
		if !disableAzureTokenVerification {
			success, _, _, _, msg = helpers.ValidateTokenForAzure(clientToken)
		} else {
			success = true
			msg = "Azure Token Validation is Disabled"
		}
		reqID, _ := c.Get("request_id")
		if success {
			var user models.User
			res := database.DB.Where("id= ?", auth.UserID).Find(&user)
			if res.RowsAffected != 0 && user.IsDeleted {
				c.JSON(http.StatusNotFound, gin.H{
					"success":    false,
					"message":    "User Not Found.",
					"request_id": requestID,
				})
				logger.Infoln("The User Is Deleted for request id", requestID, "Token", clientToken, "Username", user.Email, "and", user.ID)
				c.Abort()
				return
			}
			roles := FetchPermissions(user.ID.String())
			logger.Debugln("Token Verified for request ID", requestID, "Token", clientToken, "Username", user.Email, "with roles attached", roles)
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"claims": gin.H{
					"name":        user.Name,
					"email":       user.Email,
					"id":          user.ID,
					"type":        user.Type,
					"permissions": roles,
					"request_id":  reqID,
				},
				"message": "Token Verification Done Successfully.",
			})
		}
		if !success {
			logger.Debugln("Token Verification failed for request id", requestID, "Token", clientToken, "LoginType", auth.LoginType)
			c.JSON(http.StatusUnauthorized, gin.H{"success": success, "message": msg, "request_id": requestID})
			c.Abort()
			return
		}
	}

	if auth.LoginType == "PASSWORD" {
		claims, err := helpers.ValidateToken(c, clientToken)
		if err != "" {
			logger.Debugln("Token Verification failed for request id", requestID, "Token", clientToken, "LoginType", auth.LoginType)
			c.SetCookie("token", "", -1, "/", "", true, true)
			c.JSON(http.StatusUnauthorized, gin.H{"msg": err, "request_id": requestID})
			c.Abort()
			return
		}
		if claims == nil {
			logger.Debugln("Token Verification failed for request id", requestID, "Token", clientToken, "LoginType", auth.LoginType)
			logger.Debugln("No claims found.")
			c.SetCookie("token", "", -1, "/", "", true, true)
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "No Claims Found", "request_id": requestID})
			c.Abort()
			return
		}
		reqID, _ := c.Get("request_id")
		roles := FetchPermissions(claims.ID)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"claims": gin.H{
				"name":        claims.Name,
				"email":       claims.Email,
				"id":          claims.ID,
				"type":        claims.UserType,
				"permissions": roles,
				"request_id":  reqID,
			},
			"message": "Token Verification Done Successfully.",
		})
	}
}

// FetchPermissions Fetches permissions for individual user
func FetchPermissions(userID string) map[string]interface{} {
	logger := logger.CreateSugaredLogger("", logger.CreateCustomLogger(false, false, false))
	var project models.Project
	var memberProjects []models.Project
	var adminProjects []models.Project

	type R map[string]interface{}
	var users []string
	var admins []string

	res := database.DB.Select("name").Where("project_members @> ARRAY[?]", userID).Find(&memberProjects)
	res = database.DB.Select("name").Where("project_admins @> ?", append(project.ProjectAdmins, userID)).Find(&adminProjects)

	if res.RowsAffected == 0 {
		logger.Info("User has no DevOps_Lead rights.")
	}

	for i := 0; i < (len(memberProjects)); i++ {
		users = append(users, memberProjects[i].Name)
	}

	for i := 0; i < (len(adminProjects)); i++ {
		admins = append(admins, adminProjects[i].Name)
	}

	roles := R{
		"DevOps":      users,
		"DevOps_Lead": admins,
	}
	logger.Info("User Permissions :", roles)
	return roles
}

// ValidateTokenInAuthTable Function to Validate that token in present in auth able or not
func ValidateTokenInAuthTable(token string) bool {
	auth := models.Auth{}
	authRes := database.DB.Where("token = ?", token).Find(&auth)
	if authRes.RowsAffected != 0 {
		return true
	}
	return false
}




// /home/cloud-ops/Downloads/vcollab-backend/user_microservice/user/routers/user.go


package routers

import (
	"github.com/amartyaa/repas/user_microservice/user/controllers"
	"github.com/gin-gonic/gin"
)

// UserRouters function creates all the routers which are used for user workflow
func UserRouters(r *gin.RouterGroup) {
	userRouter := r.Group("/users").OPTIONS("/users")
	{
		userRouter.GET("", controllers.ListUsers)
		userRouter.OPTIONS("/:user_id").GET("/:user_id", controllers.GetUser)
		userRouter.PUT("/:user_id", controllers.UpdateUser)
		userRouter.DELETE("/:user_id", controllers.DeleteUser)
		userRouter.POST("", controllers.CreateUser)
		userRouter.POST("/resetpass", controllers.ResetPassword)
		userRouter.GET("/:user_id/userstats", controllers.GetUserStats)
	}

}









