package controllers

import (
	"fmt"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"profhit-backend/services"

	"github.com/gin-gonic/gin"
)

// GetAllUsers returns a paginated list of all users (admin/super_admin/it_support)
func GetAllUsers(c *gin.Context) {
	var users []models.User
	query := config.DB.Select("id, username, email, tier, role, is_active, kyc_status, points, created_at")

	// Optional search by username or email
	if search := c.Query("search"); search != "" {
		query = query.Where("username ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	// Optional filter by role
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	// Optional filter by status
	if status := c.Query("status"); status == "banned" {
		query = query.Where("is_active = false")
	} else if status == "active" {
		query = query.Where("is_active = true")
	}

	if err := query.Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users, "total": len(users)})
}

// UpdateUserRole changes a user's role. Only super_admin can do this.
func UpdateUserRole(c *gin.Context) {
	targetID := c.Param("id")

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role value
	validRoles := map[string]bool{
		models.RoleSuperAdmin:     true,
		models.RoleAdmin:          true,
		models.RoleContentCreator: true,
		models.RoleITSupport:      true,
		models.RoleUser:           true,
	}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role. Must be one of: super_admin, admin, content_creator, it_support, user"})
		return
	}

	// Prevent modifying another super_admin (safety guard)
	var target models.User
	if err := config.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	callerRole, _ := c.Get("role")
	if target.Role == models.RoleSuperAdmin && callerRole != models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot modify a super_admin's role"})
		return
	}

	if err := config.DB.Model(&target).Update("role", req.Role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	callerID := c.MustGet("userID").(uint)
	ip := c.ClientIP()
	_ = services.LogAction(nil, callerID, "UPDATE_ROLE", fmt.Sprintf("user_%d", target.ID), fmt.Sprintf("Changed role from %s to %s", target.Role, req.Role), ip)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Role updated successfully",
		"user_id":     target.ID,
		"username":    target.Username,
		"new_role":    req.Role,
	})
}

// BanUser sets is_active = false on a user
func BanUser(c *gin.Context) {
	targetID := c.Param("id")
	callerID := c.MustGet("userID").(uint)
	callerRole, _ := c.Get("role")

	var target models.User
	if err := config.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Prevent self-ban
	if target.ID == callerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot ban yourself"})
		return
	}
	// super_admin cannot be banned
	if target.Role == models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot ban a super_admin"})
		return
	}
	// An admin cannot ban another admin — only super_admin can
	if target.Role == models.RoleAdmin && callerRole != models.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only super_admin can ban another admin"})
		return
	}

	if err := config.DB.Model(&target).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ban user"})
		return
	}
	
	ip := c.ClientIP()
	_ = services.LogAction(nil, callerID, "BAN_USER", fmt.Sprintf("user_%d", target.ID), "Banned user", ip)
	
	c.JSON(http.StatusOK, gin.H{"message": "User banned successfully", "user_id": target.ID, "username": target.Username})
}

// UnbanUser sets is_active = true on a user
func UnbanUser(c *gin.Context) {
	targetID := c.Param("id")

	var target models.User
	if err := config.DB.First(&target, targetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := config.DB.Model(&target).Update("is_active", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unban user"})
		return
	}

	callerID := c.MustGet("userID").(uint)
	ip := c.ClientIP()
	_ = services.LogAction(nil, callerID, "UNBAN_USER", fmt.Sprintf("user_%d", target.ID), "Unbanned user", ip)

	c.JSON(http.StatusOK, gin.H{"message": "User unbanned successfully", "user_id": target.ID, "username": target.Username})
}


// GetPlatformStats returns aggregate admin dashboard statistics
func GetPlatformStats(c *gin.Context) {
	var totalUsers int64
	var activeUsers int64
	var bannedUsers int64
	var totalMarkets int64
	var openMarkets int64
	var totalTrades int64

	config.DB.Model(&models.User{}).Count(&totalUsers)
	config.DB.Model(&models.User{}).Where("is_active = true").Count(&activeUsers)
	config.DB.Model(&models.User{}).Where("is_active = false").Count(&bannedUsers)
	config.DB.Model(&models.Market{}).Count(&totalMarkets)
	config.DB.Model(&models.Market{}).Where("resolution_status = ?", "Open").Count(&openMarkets)
	config.DB.Model(&models.PredictionSubmission{}).Count(&totalTrades)

	// Total volume traded (n/a for fixed-odds predictions)
	var totalVolume struct{ Sum float64 }

	// Total wallets balance
	var totalWalletBalance int64
	config.DB.Model(&models.User{}).Select("COALESCE(SUM(points), 0)").Row().Scan(&totalWalletBalance)

	// Role breakdown
	type RoleCount struct {
		Role  string
		Count int64
	}
	var roleCounts []RoleCount
	config.DB.Model(&models.User{}).
		Select("role, count(*) as count").
		Group("role").
		Scan(&roleCounts)

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"total":   totalUsers,
			"active":  activeUsers,
			"banned":  bannedUsers,
			"by_role": roleCounts,
		},
		"markets": gin.H{
			"total": totalMarkets,
			"open":  openMarkets,
		},
		"trades": gin.H{
			"total":        totalTrades,
			"total_volume": totalVolume.Sum,
		},
		"wallets": gin.H{
			"total_balance": totalWalletBalance,
		},
	})
}
