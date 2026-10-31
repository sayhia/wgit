import { describe, expect, test } from "bun:test";
import { mount } from "@vue/test-utils";
import SettingsCard from "../src/components/settings/SettingsCard.vue";
import SettingsPageHeader from "../src/components/settings/SettingsPageHeader.vue";
import "./domSetup";

test("SettingsCard renders header/actions/default slots", () => {
  const wrapper = mount(SettingsCard, {
    slots: { header: "<span>Header</span>", actions: "<button>Act</button>", default: "<p>Body</p>" },
  });
  expect(wrapper.text()).toContain("Header");
  expect(wrapper.text()).toContain("Act");
  expect(wrapper.text()).toContain("Body");
  expect(wrapper.find("section").exists()).toBe(true);
});

test("SettingsPageHeader renders one heading and description", () => {
  const wrapper = mount(SettingsPageHeader, {
    props: { title: "AI 连接", description: "配置 API 或 CLI" },
  });
  expect(wrapper.find("h2").text()).toBe("AI 连接");
  expect(wrapper.text()).toContain("配置 API 或 CLI");
});
