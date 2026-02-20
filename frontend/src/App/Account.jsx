import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ExitIcon } from "@radix-ui/react-icons";
import { Key, RefreshCw, Shield, LogOut } from "lucide-react";

const Account = () => {
  const navigate = useNavigate();
  const state = GLOBAL_STATE("account");

  const gotoAccountSelect = () => {
    navigate("/accounts");
  };

  if (state.User?.Email === "" || !state.User) {
    gotoAccountSelect();
    return;
  }

  useEffect(() => {
    state.GetBackendState();
  }, []);

  state.User?.Tokens?.sort(function (x, y) {
    if (x.Created < y.Created) return 1;
    if (x.Created > y.Created) return -1;
    return 0;
  });

  let APIKey = state?.User?.APIKey;
  const [tab, setTab] = React.useState("account");

  const tabs = [
    { key: "account", label: "Account" },
    { key: "loggedin", label: "Devices" },
    { key: "license", label: "License Key" },
  ];

  return (
    <div>
      {/* ── Tab bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] mb-6">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                tab === t.key ? "bg-white/[0.07] text-white/70" : "text-white/40 hover:text-white/60"
              }`}
              onClick={() => setTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* ── Account tab ── */}
      {tab === "account" && state?.User && (
        <div className="max-w-lg">
          <div className="space-y-px mb-6">
            {[
              { label: "User", value: state.User?.Email },
              { label: "ID", value: state.User?._id },
              { label: "Updated", value: state.User?.Updated ? dayjs(state.User.Updated).format("DD-MM-YYYY HH:mm:ss") : "—" },
              state.User?.SubExpiration && { label: "Subscription", value: dayjs(state.User.SubExpiration).format("DD-MM-YYYY HH:mm:ss") },
              { label: "API Key", value: APIKey },
              state.User?.Trial && { label: "Trial", value: state.User?.Trial ? "Active" : "Ended" },
            ]
              .filter(Boolean)
              .map((item, i) => (
                <div key={i} className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-blue-500/20 hover:border-blue-500/50 transition-colors">
                  <span className="text-[11px] text-white/45 shrink-0 w-[100px]">{item.label}</span>
                  <code className="text-[13px] text-white/60 font-mono truncate">{item.value ?? "—"}</code>
                </div>
              ))}
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-[11px] text-white/40 hover:text-white/70 hover:bg-white/[0.04] border border-white/[0.06]"
              onClick={() => gotoAccountSelect()}
            >
              Switch Account
            </Button>
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-[11px] text-white/40 hover:text-white/70 hover:bg-white/[0.04] border border-white/[0.06]"
              onClick={() => state.refreshApiKey()}
            >
              <RefreshCw className="h-3 w-3 mr-1" /> Re-Generate API Key
            </Button>
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-[11px] text-white/40 hover:text-white/70 hover:bg-white/[0.04] border border-white/[0.06]"
              onClick={() => navigate("/twofactor/create")}
            >
              <Shield className="h-3 w-3 mr-1" /> Two-Factor Auth
            </Button>
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-[11px] text-red-400/60 hover:text-red-400 hover:bg-red-500/[0.04] border border-red-500/10"
              onClick={() => state.LogoutAllTokens()}
            >
              <LogOut className="h-3 w-3 mr-1" /> Log Out All Devices
            </Button>
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-[11px] text-red-400/60 hover:text-red-400 hover:bg-red-500/[0.04] border border-red-500/10"
              onClick={() => {
                let t = state.User?.DeviceToken;
                if (t !== "") state.LogoutToken(t, false);
              }}
            >
              <LogOut className="h-3 w-3 mr-1" /> Logout
            </Button>
          </div>
        </div>
      )}

      {/* ── Devices tab ── */}
      {tab === "loggedin" && (
        <div>
          <div className="flex items-center gap-4 pl-3 border-l-2 border-transparent mb-1">
            <span className="text-[10px] text-white/40 uppercase tracking-wider flex-1 min-w-0">Device</span>
            <span className="text-[10px] text-white/40 uppercase tracking-wider shrink-0 w-36 text-right">Created</span>
            <span className="shrink-0 w-16" />
          </div>
          <div className="space-y-px">
            {state.User?.Tokens?.length > 0 ? state.User.Tokens.map((t, i) => {
              const isCurrent = t.DT === state?.User?.DeviceToken?.DT;
              return (
                <div key={i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-cyan-500/20 hover:border-cyan-500/50 transition-colors">
                  <div className="flex-1 min-w-0">
                    <span className="text-[13px] text-white/80 font-medium truncate block">
                      {t.N}{isCurrent ? " (current)" : ""}
                    </span>
                  </div>
                  <span className="text-[11px] text-white/40 tabular-nums shrink-0 w-36 text-right">
                    {t.Created ? dayjs(t.Created).format("HH:mm:ss DD-MM-YYYY") : "—"}
                  </span>
                  <div className="shrink-0 w-16 flex justify-end opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      variant="ghost"
                      onClick={() => state.LogoutToken(t, false)}
                      className="h-6 px-2 text-red-500/60 hover:text-red-400 text-[11px] gap-1"
                    >
                      <ExitIcon className="w-3 h-3" /> Logout
                    </Button>
                  </div>
                </div>
              );
            }) : (
              <div className="py-6 pl-3 border-l-2 border-white/[0.04] text-[12px] text-white/40">No active sessions</div>
            )}
          </div>
        </div>
      )}

      {/* ── License tab ── */}
      {tab === "license" && (
        <div className="max-w-lg space-y-4">
          {state.User.Key?.Key && (
            <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-amber-500/20">
              <span className="text-[11px] text-white/45 shrink-0 w-[60px]">Current</span>
              <code className="text-[13px] text-white/60 font-mono truncate">{state.User.Key.Key}</code>
            </div>
          )}

          <div className="space-y-2">
            <label className="text-[10px] text-white/50 uppercase block">Activate License Key</label>
            <div className="flex items-center gap-2">
              <Input
                className="h-7 text-[12px] border-[#1e2433] bg-transparent flex-1"
                onChange={(e) => state.UpdateLicenseInput(e.target.value)}
                placeholder="Insert License Key"
                value={state.LicenseKey}
              />
              <Button
                className="text-white bg-emerald-600 hover:bg-emerald-500 h-7 text-[11px] px-3"
                onClick={() => state.ActivateLicense()}
              >
                <Key className="h-3 w-3 mr-1" /> Activate
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Account;
