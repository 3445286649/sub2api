import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import SubscriptionPlanCard from "../SubscriptionPlanCard.vue";

const i18n = createI18n({
  legacy: false,
  locale: "en",
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      payment: {
        days: "days",
        models: "Models",
        planCard: {
          quota: "Quota",
          rate: "Rate",
          unlimited: "Unlimited",
        },
        quotaReset: {
          buyReset: "Buy Reset",
          daily: "Daily quota",
          once: "time",
          resetValue: "Reset Value",
          typeLabel: "Reset",
        },
        renewNow: "Renew",
        subscribeNow: "Subscribe now",
      },
    },
  },
});

const mountPlanCard = (groupPlatform: string) =>
  mount(SubscriptionPlanCard, {
    props: {
      plan: {
        id: 1,
        group_id: 10,
        group_platform: groupPlatform,
        plan_type: "subscription",
        name: "Pro",
        price: 10,
        amount: 1000,
        features: [],
        rate_multiplier: 1,
        validity_days: 30,
        validity_unit: "day",
        supported_model_scopes: ["claude", "gemini_text", "gemini_image"],
        is_active: true,
      },
    },
    global: { plugins: [i18n, createPinia()] },
  });

describe("SubscriptionPlanCard", () => {
  it("does not show Antigravity model scopes for OpenAI plans", () => {
    const text = mountPlanCard("openai").text();

    expect(text).not.toContain("Claude");
    expect(text).not.toContain("Gemini");
    expect(text).not.toContain("Imagen");
  });

  it("shows model scopes for Antigravity plans", () => {
    const text = mountPlanCard("antigravity").text();

    expect(text).toContain("Claude");
    expect(text).toContain("Gemini");
    expect(text).toContain("Imagen");
  });

  it("uses reset copy for quota reset plans", () => {
    const wrapper = mount(SubscriptionPlanCard, {
      props: {
        plan: {
          id: 2,
          group_id: 10,
          group_platform: "openai",
          plan_type: "quota_reset",
          name: "Daily Reset 100",
          price: 3,
          description: "",
          features: [],
          rate_multiplier: 1,
          validity_days: 1,
          validity_unit: "days",
          quota_reset_scope: "daily",
          quota_reset_value: 100,
          for_sale: true,
          sort_order: 0,
        },
      },
      global: { plugins: [i18n] },
    });

    const text = wrapper.text();
    expect(text).toContain("payment.quotaReset.buyReset");
    expect(text).toContain("payment.quotaReset.resetValue");
    expect(text).not.toContain("Subscribe now");
    expect(text).not.toContain("Renew");
  });
});
