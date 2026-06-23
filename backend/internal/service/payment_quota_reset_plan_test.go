//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCreatePlanDefaultsToSubscriptionPlanType(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil)

	plan, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:      10,
		Name:         "Daily 100",
		Description:  "normal subscription",
		Price:        19.9,
		ValidityDays: 1,
		ValidityUnit: "days",
		ForSale:      true,
	})

	require.NoError(t, err)
	require.Equal(t, SubscriptionPlanTypeSubscription, plan.PlanType)
	require.Empty(t, plan.QuotaResetScope)
	require.Zero(t, plan.QuotaResetValue)
}

func TestCreateQuotaResetPlanValidatesScopeAndValue(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil)

	_, err := svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:         10,
		PlanType:        SubscriptionPlanTypeQuotaReset,
		Name:            "Weekly reset",
		Description:     "unsupported scope",
		Price:           9.9,
		ValidityDays:    1,
		ValidityUnit:    "days",
		QuotaResetScope: QuotaResetScopeWeekly,
		QuotaResetValue: 100,
		ForSale:         true,
	})

	require.Error(t, err)
	require.Equal(t, "INVALID_QUOTA_RESET_SCOPE", infraerrors.Reason(err))

	_, err = svc.CreatePlan(ctx, CreatePlanRequest{
		GroupID:         10,
		PlanType:        SubscriptionPlanTypeQuotaReset,
		Name:            "Daily reset",
		Description:     "missing value",
		Price:           9.9,
		ValidityDays:    1,
		ValidityUnit:    "days",
		QuotaResetScope: QuotaResetScopeDaily,
		QuotaResetValue: 0,
		ForSale:         true,
	})

	require.Error(t, err)
	require.Equal(t, "QUOTA_RESET_VALUE_INVALID", infraerrors.Reason(err))
}

func TestValidateSubOrderRejectsQuotaResetWithoutMatchingActiveSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(7).
		SetPlanType(SubscriptionPlanTypeQuotaReset).
		SetName("Daily Reset 100").
		SetDescription("quota reset").
		SetPrice(8.8).
		SetValidityDays(1).
		SetValidityUnit("days").
		SetQuotaResetScope(QuotaResetScopeDaily).
		SetQuotaResetValue(100).
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	dailyLimit := 100.0
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit},
	}
	subscriptionSvc := NewSubscriptionService(groupRepo, newSubscriptionUserSubRepoStub(), nil, nil, nil)
	svc := &PaymentService{
		configService:   NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil),
		groupRepo:       groupRepo,
		subscriptionSvc: subscriptionSvc,
	}

	_, err = svc.validateSubOrder(ctx, CreateOrderRequest{UserID: 10, PlanID: plan.ID})

	require.ErrorIs(t, err, ErrQuotaResetValueExceedsLimit)
}

func TestValidateSubOrderRejectsQuotaResetAboveCurrentDailyLimit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	plan, err := client.SubscriptionPlan.Create().
		SetGroupID(9).
		SetPlanType(SubscriptionPlanTypeQuotaReset).
		SetName("Daily Reset 200").
		SetDescription("quota reset").
		SetPrice(12.8).
		SetValidityDays(1).
		SetValidityUnit("days").
		SetQuotaResetScope(QuotaResetScopeDaily).
		SetQuotaResetValue(200).
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	daily100 := 100.0
	daily200 := 200.0
	groupRepo := &quotaResetGroupRepoStub{groups: map[int64]*Group{
		7: {ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &daily100},
		9: {ID: 9, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &daily200},
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:        88,
		UserID:    10,
		GroupID:   7,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	subscriptionSvc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	svc := &PaymentService{
		configService:   NewPaymentConfigService(client, &paymentFulfillmentSettingRepoStub{}, nil),
		groupRepo:       groupRepo,
		subscriptionSvc: subscriptionSvc,
	}

	_, err = svc.validateSubOrder(ctx, CreateOrderRequest{UserID: 10, PlanID: plan.ID})

	require.ErrorIs(t, err, ErrQuotaResetValueExceedsLimit)
}

func TestExecuteSubscriptionFulfillmentQuotaResetDeductsUsageWithoutExtending(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("quota-reset-payment@example.com").
		SetPasswordHash("hash").
		SetUsername("quota-reset-payment-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(8.8).
		SetPayAmount(8.8).
		SetFeeRate(0).
		SetRechargeCode("PAY-QUOTA-RESET").
		SetOutTradeNo("sub2_quota_reset_payment").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-quota-reset").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(200).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(1).
		SetSubscriptionPlanType(SubscriptionPlanTypeQuotaReset).
		SetQuotaResetScope(QuotaResetScopeDaily).
		SetQuotaResetValue(100).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	dailyLimit := 200.0
	expiresAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:              77,
		UserID:          user.ID,
		GroupID:         7,
		Status:          SubscriptionStatusActive,
		ExpiresAt:       expiresAt,
		DailyUsageUSD:   180,
		WeeklyUsageUSD:  180,
		MonthlyUsageUSD: 180,
	})
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit},
	}
	subscriptionSvc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: subscriptionSvc,
	}

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Zero(t, subRepo.createCalls)

	sub, err := subRepo.GetByID(ctx, 77)
	require.NoError(t, err)
	require.Equal(t, 80.0, sub.DailyUsageUSD)
	require.Equal(t, 80.0, sub.WeeklyUsageUSD)
	require.Equal(t, 80.0, sub.MonthlyUsageUSD)
	require.True(t, sub.ExpiresAt.Equal(expiresAt))

	audit, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("PAYMENT_QUOTA_RESET_SUCCESS")).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, audit.Detail, `"subscription_id":77`)
	require.Contains(t, audit.Detail, `"group_id":7`)
	require.Contains(t, audit.Detail, `"scope":"daily"`)
	require.Contains(t, audit.Detail, `"reset_value":100`)
}

func TestExecuteSubscriptionFulfillmentQuotaResetIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)

	user, err := client.User.Create().
		SetEmail("quota-reset-idempotent@example.com").
		SetPasswordHash("hash").
		SetUsername("quota-reset-idempotent-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(8.8).
		SetPayAmount(8.8).
		SetFeeRate(0).
		SetRechargeCode("PAY-QUOTA-RESET-IDEMPOTENT").
		SetOutTradeNo("sub2_quota_reset_idempotent").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-quota-reset-idempotent").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(201).
		SetSubscriptionGroupID(7).
		SetSubscriptionDays(1).
		SetSubscriptionPlanType(SubscriptionPlanTypeQuotaReset).
		SetQuotaResetScope(QuotaResetScopeDaily).
		SetQuotaResetValue(100).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("PAYMENT_QUOTA_RESET_SUCCESS").
		SetDetail(`{"subscription_id":77,"group_id":7,"scope":"daily","reset_value":100}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)

	dailyLimit := 100.0
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:             77,
		UserID:         user.ID,
		GroupID:        7,
		Status:         SubscriptionStatusActive,
		DailyUsageUSD:  60,
		WeeklyUsageUSD: 60,
	})
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, Status: payment.EntityStatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit},
	}
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: NewSubscriptionService(groupRepo, subRepo, nil, nil, nil),
	}

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	sub, err := subRepo.GetByID(ctx, 77)
	require.NoError(t, err)
	require.Equal(t, 60.0, sub.DailyUsageUSD)
	require.Equal(t, 60.0, sub.WeeklyUsageUSD)
}
