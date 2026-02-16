package services

import (
	"context"
	"fmt"

	"github.com/ads-marketplace/backend/internal/models"
	"github.com/ads-marketplace/backend/internal/repositories"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CampaignService struct {
	campaignRepo *repositories.CampaignRepo
	auditRepo    *repositories.AuditRepo
	log          *zap.Logger
}

func NewCampaignService(
	campaignRepo *repositories.CampaignRepo,
	auditRepo *repositories.AuditRepo,
	log *zap.Logger,
) *CampaignService {
	return &CampaignService{
		campaignRepo: campaignRepo,
		auditRepo:    auditRepo,
		log:          log,
	}
}

func (s *CampaignService) Create(ctx context.Context, userID uuid.UUID, c *models.Campaign) error {
	c.AdvertiserUserID = userID
	if c.Status == "" {
		c.Status = "active"
	}

	if err := s.campaignRepo.Create(ctx, c); err != nil {
		return err
	}

	_ = s.auditRepo.Log(ctx, models.AuditLog{
		ActorUserID: &userID,
		ActorType:   "user",
		Action:      "campaign_created",
		EntityType:  "campaign",
		EntityID:    &c.ID,
	})

	return nil
}

func (s *CampaignService) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*models.Campaign, error) {
	c, err := s.campaignRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.AdvertiserUserID != userID {
		return nil, fmt.Errorf("campaign not found")
	}
	return c, nil
}

func (s *CampaignService) List(ctx context.Context, userID uuid.UUID, f repositories.CampaignFilter) ([]models.Campaign, error) {
	f.AdvertiserUserID = &userID
	return s.campaignRepo.List(ctx, f)
}

func (s *CampaignService) Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, c *models.Campaign) error {
	existing, err := s.campaignRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("campaign not found")
	}
	if existing.AdvertiserUserID != userID {
		return fmt.Errorf("campaign not found")
	}

	c.ID = id
	c.AdvertiserUserID = existing.AdvertiserUserID
	if c.Status == "" {
		c.Status = existing.Status
	}

	return s.campaignRepo.Update(ctx, c)
}

func (s *CampaignService) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	existing, err := s.campaignRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("campaign not found")
	}
	if existing.AdvertiserUserID != userID {
		return fmt.Errorf("campaign not found")
	}

	return s.campaignRepo.Delete(ctx, id)
}
