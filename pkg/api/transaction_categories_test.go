package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestCreateNewCategoryModelIncludesAIGuidance(t *testing.T) {
	request := &models.TransactionCategoryCreateRequest{
		Name:       "Transport",
		Type:       models.CATEGORY_TYPE_EXPENSE,
		AiGuidance: "Use for Udrive and taxi transactions.",
	}

	category := TransactionCategories.createNewCategoryModel(1, request, 2)

	assert.Equal(t, request.AiGuidance, category.AiGuidance)
}
