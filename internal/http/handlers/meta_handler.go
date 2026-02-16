package handlers

import (
	"github.com/ads-marketplace/backend/internal/http/dto"
	"github.com/gofiber/fiber/v2"
)

type MetaHandler struct{}

func NewMetaHandler() *MetaHandler {
	return &MetaHandler{}
}

type MetaCategory struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type MetaLanguage struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var predefinedCategories = []MetaCategory{
	{ID: "crypto", Label: "Crypto & Web3"},
	{ID: "news", Label: "News & Media"},
	{ID: "tech", Label: "Technology"},
	{ID: "finance", Label: "Finance & Trading"},
	{ID: "education", Label: "Education"},
	{ID: "entertainment", Label: "Entertainment"},
	{ID: "lifestyle", Label: "Lifestyle"},
	{ID: "gaming", Label: "Gaming"},
	{ID: "marketing", Label: "Marketing & SMM"},
	{ID: "sports", Label: "Sports"},
	{ID: "travel", Label: "Travel"},
	{ID: "food", Label: "Food & Cooking"},
	{ID: "health", Label: "Health & Fitness"},
	{ID: "music", Label: "Music"},
	{ID: "art", Label: "Art & Design"},
	{ID: "science", Label: "Science"},
	{ID: "politics", Label: "Politics"},
	{ID: "business", Label: "Business"},
	{ID: "other", Label: "Other"},
}

var predefinedLanguages = []MetaLanguage{
	{ID: "en", Label: "English"},
	{ID: "ru", Label: "Русский"},
	{ID: "uk", Label: "Українська"},
	{ID: "zh", Label: "中文"},
	{ID: "es", Label: "Español"},
	{ID: "ar", Label: "العربية"},
	{ID: "hi", Label: "हिन्दी"},
	{ID: "pt", Label: "Português"},
	{ID: "fr", Label: "Français"},
	{ID: "de", Label: "Deutsch"},
	{ID: "ja", Label: "日本語"},
	{ID: "ko", Label: "한국어"},
	{ID: "tr", Label: "Türkçe"},
	{ID: "id", Label: "Bahasa Indonesia"},
	{ID: "vi", Label: "Tiếng Việt"},
	{ID: "it", Label: "Italiano"},
	{ID: "th", Label: "ไทย"},
	{ID: "fa", Label: "فارسی"},
	{ID: "pl", Label: "Polski"},
	{ID: "uz", Label: "Oʻzbekcha"},
	{ID: "other", Label: "Other"},
}

func (h *MetaHandler) GetCategories(c *fiber.Ctx) error {
	return c.JSON(dto.SuccessResponse{OK: true, Data: predefinedCategories})
}

func (h *MetaHandler) GetLanguages(c *fiber.Ctx) error {
	return c.JSON(dto.SuccessResponse{OK: true, Data: predefinedLanguages})
}
