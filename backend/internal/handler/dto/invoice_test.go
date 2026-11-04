//go:build unit

package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func invoiceDTOFixture() *service.InvoiceRequest {
	encrypted, masked, plain, notes := "ciphertext", "3301**********123X", "33010619900101123X", "internal note"
	return &service.InvoiceRequest{
		ID: 1, UserID: 7, UserEmail: "u@example.com", InvoiceType: service.InvoiceTypeIndividual, Title: "t",
		IDNumberEncrypted: &encrypted, IDNumberMasked: &masked, IDNumberPlain: &plain, AdminNotes: &notes,
	}
}

func TestUserInvoiceResponseHidesAdminOnlyFields(t *testing.T) {
	raw, err := json.Marshal(InvoiceRequestFromService(invoiceDTOFixture()))
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "3301**********123X")
	require.NotContains(t, body, "33010619900101123X")
	require.NotContains(t, body, "ciphertext")
	require.NotContains(t, body, "internal note")
	require.NotContains(t, body, "u@example.com")
}

func TestAdminInvoiceResponseShowsDecryptedIDAndNotes(t *testing.T) {
	raw, err := json.Marshal(AdminInvoiceRequestFromService(invoiceDTOFixture()))
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "33010619900101123X", decoded["individual_id_number"])
	require.Equal(t, "internal note", decoded["admin_notes"])
	require.Equal(t, "u@example.com", decoded["user_email"])
	require.NotContains(t, string(raw), "ciphertext")
}

func TestAdminInvoiceResponseFallsBackToMaskedID(t *testing.T) {
	req := invoiceDTOFixture()
	req.IDNumberPlain = nil
	require.Equal(t, "3301**********123X", *AdminInvoiceRequestFromService(req).IndividualIDNumber)
}
