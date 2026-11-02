import { flushPromises, mount, config } from "@vue/test-utils";
import { beforeEach, describe, expect, test, vi } from "vite-plus/test";
import OcppMaintenanceModal from "./OcppMaintenanceModal.vue";
import en from "../../../../i18n/en.json";

const request = vi.fn();
vi.mock("@/api", () => ({ default: { request: (...args: unknown[]) => request(...args) } }));
vi.mock("@/configModal", () => ({
  getModal: () => ({ name: "ocppmaintenance", station: "station/1" }),
}));

const lookup = (key: string): string | undefined => {
  const v = key.split(".").reduce<any>((o, k) => o?.[k], en);
  return typeof v === "string" ? v : undefined;
};
config.global.mocks["$t"] = (key: string, params?: Record<string, unknown>) =>
  (lookup(key) ?? key).replace(/\{(\w+)\}/g, (_, p) => String(params?.[p] ?? `{${p}}`));

const GenericModal = {
  emits: ["open", "closed"],
  template: "<div><slot /></div>",
};

const station = (transactionId?: number) => ({
  id: "station/1",
  status: "connected",
  vendor: "go-e",
  model: "Gemini",
  firmwareVersion: "59.4",
  connectors: [{ id: 1, status: "Charging", transactionId, idTag: transactionId ? "tag" : "" }],
});

const open = async () => {
  const wrapper = mount(OcppMaintenanceModal, { global: { stubs: { GenericModal } } });
  wrapper.findComponent(GenericModal).vm.$emit("open");
  await flushPromises();
  return wrapper;
};

const button = (wrapper: Awaited<ReturnType<typeof open>>, text: string) =>
  wrapper.findAll("button").find((b) => b.text() === text)!;

describe("OcppMaintenanceModal", () => {
  beforeEach(() => {
    request.mockReset();
  });

  test("shows station info", async () => {
    request.mockResolvedValue({ status: 200, data: station(42) });
    const wrapper = await open();

    expect(request).toHaveBeenCalledWith(
      expect.objectContaining({ method: "get", url: "/config/ocpp/stations/station%2F1" })
    );
    const info = wrapper.find("[data-testid=ocppmaintenance-info]").text();
    expect(info).toContain("go-e");
    expect(info).toContain("59.4");
    expect(wrapper.text()).toContain("Charging session 42, ID tag tag");
    wrapper.unmount();
  });

  test("stop requires known transaction", async () => {
    request.mockResolvedValue({ status: 200, data: station() });
    const wrapper = await open();

    expect(wrapper.text()).toContain("No known charging session");
    expect(button(wrapper, "Stop charging").attributes("disabled")).toBeDefined();
    expect(button(wrapper, "Unlock connector").attributes("disabled")).toBeUndefined();
    wrapper.unmount();
  });

  test("confirms before sending", async () => {
    request.mockResolvedValue({ status: 200, data: station(42) });
    const wrapper = await open();
    const calls = request.mock.calls.length;

    await button(wrapper, "Soft reset").trigger("click");
    expect(wrapper.find("[data-testid=ocppmaintenance-confirm]").exists()).toBe(true);
    expect(request).toHaveBeenCalledTimes(calls);

    await button(wrapper, "Cancel").trigger("click");
    expect(wrapper.find("[data-testid=ocppmaintenance-confirm]").exists()).toBe(false);
    expect(request).toHaveBeenCalledTimes(calls);
    wrapper.unmount();
  });

  test("shows charger rejection", async () => {
    request.mockResolvedValue({ status: 200, data: station(42) });
    const wrapper = await open();

    await button(wrapper, "Unlock connector").trigger("click");
    request.mockResolvedValueOnce({ status: 422, data: { error: "UnlockFailed" } });
    await wrapper.find("[data-testid=ocppmaintenance-confirm] .btn-primary").trigger("click");
    await flushPromises();

    expect(request).toHaveBeenCalledWith(
      expect.objectContaining({
        method: "post",
        url: "/config/ocpp/stations/station%2F1/connectors/1/unlock",
      })
    );
    const result = wrapper.find("[data-testid=ocppmaintenance-result]");
    expect(result.text()).toBe("UnlockFailed");
    expect(result.classes()).toContain("alert-danger");
    wrapper.unmount();
  });

  test("hard reset", async () => {
    request.mockResolvedValue({ status: 200, data: station(42) });
    const wrapper = await open();

    await button(wrapper, "Hard reset").trigger("click");
    await wrapper.find("[data-testid=ocppmaintenance-confirm] .btn-danger").trigger("click");
    await flushPromises();

    expect(request).toHaveBeenCalledWith(
      expect.objectContaining({
        method: "post",
        url: "/config/ocpp/stations/station%2F1/reset/hard",
      })
    );
    expect(wrapper.find("[data-testid=ocppmaintenance-result]").classes()).toContain(
      "alert-success"
    );
    wrapper.unmount();
  });

  test("shows ocpp log", async () => {
    request.mockImplementation(({ url }: { url: string }) =>
      Promise.resolve(
        url.endsWith("/log")
          ? {
              status: 200,
              data: [
                {
                  time: "2026-11-02T09:52:15Z",
                  direction: "recv",
                  frame: '[2,"1","BootNotification",{}]',
                },
                { time: "2026-11-02T09:52:16Z", direction: "send", frame: '[3,"1",{}]' },
              ],
            }
          : { status: 200, data: station(42) }
      )
    );
    const wrapper = await open();

    expect(request).toHaveBeenCalledWith(
      expect.objectContaining({ method: "get", url: "/config/ocpp/stations/station%2F1/log" })
    );
    const log = () => wrapper.find("[data-testid=ocppmaintenance-log]");
    // collapsed below the actions on small screens
    expect(log().classes()).toContain("d-none");
    await button(wrapper, "Show OCPP log").trigger("click");
    expect(log().classes()).not.toContain("d-none");

    expect(wrapper.find("[data-testid=ocppmaintenance-log-legend]").text()).toContain(
      "Sent to charger"
    );
    const entries = () => wrapper.findAll("[data-testid=ocppmaintenance-log] .entry");
    expect(entries()).toHaveLength(2);
    expect(entries()[0]!.text()).toContain("←");
    expect(entries()[0]!.text()).toContain("BootNotification");
    expect(entries()[1]!.text()).toContain("Response to BootNotification");

    // expanded by default
    expect(entries().every((e) => e.find("pre").exists())).toBe(true);
    expect(entries()[0]!.find("pre").text()).toBe("{}");

    // single message toggles against the default
    await entries()[0]!.find("button").trigger("click");
    expect(entries()[0]!.find("pre").exists()).toBe(false);

    await button(wrapper, "Collapse all").trigger("click");
    expect(entries().some((e) => e.find("pre").exists())).toBe(false);
    await button(wrapper, "Expand all").trigger("click");
    expect(entries().every((e) => e.find("pre").exists())).toBe(true);
    wrapper.unmount();
  });

  test("hides success after a while", async () => {
    vi.useFakeTimers();
    request.mockResolvedValue({ status: 200, data: station(42) });
    const wrapper = await open();

    await button(wrapper, "Soft reset").trigger("click");
    await wrapper.find("[data-testid=ocppmaintenance-confirm] .btn-primary").trigger("click");
    await flushPromises();
    expect(wrapper.find("[data-testid=ocppmaintenance-result]").exists()).toBe(true);

    await vi.advanceTimersByTimeAsync(5000);
    expect(wrapper.find("[data-testid=ocppmaintenance-result]").exists()).toBe(false);
    wrapper.unmount();
    vi.useRealTimers();
  });
});
