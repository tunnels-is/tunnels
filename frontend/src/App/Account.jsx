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
    { key: "loggedin", label: "Logins" },
    { key: "license", label: "License Key" },
  ];

  return (
    <div>
      {/* ── Tab bar ── */}
      <div className="flex items-center gap-5 py-3 px-4 rounded-lg bg-[#ffffff]/80 border border-[#e7e3d7] mb-6 card-shadow">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              className={`text-[11px] px-2.5 py-0.5 rounded transition-colors ${
                tab === t.key ? "bg-black/[0.05] text-[#0a0a0a]" : "text-[#a3a3a3] hover:text-[#525252]"
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
                <div key={i} className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-[#1d4ed8]/30 hover:border-[#1d4ed8]/60 transition-colors">
                  <span className="label-caption shrink-0 w-[100px]">{item.label}</span>
                  <code className="text-[13px] text-[#525252] font-mono truncate">{item.value ?? "—"}</code>
                </div>
              ))}
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              variant="ghost"
              className="btn btn-outline btn-sm"
              onClick={() => gotoAccountSelect()}
            >
              Switch Account
            </Button>
            <Button
              variant="ghost"
              className="btn btn-outline btn-sm"
              onClick={() => state.refreshApiKey()}
            >
              <RefreshCw className="h-3 w-3 mr-1" /> Re-Generate API Key
            </Button>
            <Button
              variant="ghost"
              className="btn btn-outline btn-sm"
              onClick={() => navigate("/twofactor/create")}
            >
              <Shield className="h-3 w-3 mr-1" /> Two-Factor Auth
            </Button>
            <Button
              variant="ghost"
              className="btn btn-outline-danger btn-sm"
              onClick={() => state.LogoutAllTokens()}
            >
              <LogOut className="h-3 w-3 mr-1" /> Log Out All Devices
            </Button>
            <Button
              variant="ghost"
              className="btn btn-outline-danger btn-sm"
              onClick={() => {
                let t = state.User?.DeviceToken;
                if (t?.DT) state.LogoutToken(t, false);
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
            <span className="label flex-1 min-w-0">Name</span>
            <span className="label shrink-0 w-36 text-right">Created</span>
            <span className="shrink-0 w-16" />
          </div>
          <div className="space-y-px">
            {state.User?.Tokens?.length > 0 ? state.User.Tokens.map((t, i) => {
              const isCurrent = t.DT === state?.User?.DeviceToken?.DT;
              return (
                <div key={i} className="group flex items-center gap-4 py-1.5 pl-3 border-l-2 border-[#1d4ed8]/30 hover:border-[#1d4ed8]/60 transition-colors">
                  <div className="flex-1 min-w-0">
                    <span className="text-[13px] text-[#0a0a0a] font-medium truncate block">
                      {t.N}{isCurrent ? " (current)" : ""}
                    </span>
                  </div>
                  <span className="text-[11px] text-[#a3a3a3] tabular-nums shrink-0 w-36 text-right">
                    {t.Created ? dayjs(t.Created).format("HH:mm:ss DD-MM-YYYY") : "—"}
                  </span>
                  <div className="shrink-0 w-16 flex justify-end opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button
                      variant="ghost"
                      onClick={() => state.LogoutToken(t, false)}
                      className="btn btn-ghost-danger btn-xs"
                    >
                      <ExitIcon className="w-3 h-3" /> Logout
                    </Button>
                  </div>
                </div>
              );
            }) : (
              <div className="py-6 pl-3 border-l-2 border-black/[0.04] text-[12px] text-[#a3a3a3]">No active sessions</div>
            )}
          </div>
        </div>
      )}

      {/* ── License tab ── */}
      {tab === "license" && (
        <div className="max-w-lg space-y-4">
          {state.User.Key?.Key && (
            <div className="flex items-baseline gap-3 py-1.5 pl-3 border-l-2 border-[#b45309]/30">
              <span className="label-caption shrink-0 w-[60px]">Current</span>
              <code className="text-[13px] text-[#525252] font-mono truncate">{state.User.Key.Key}</code>
            </div>
          )}

          <div className="space-y-2">
            <label className="label !mb-0">Activate License Key</label>
            <div className="flex items-center gap-2">
              <Input
                className="h-7 text-[12px] border-[#e7e3d7] bg-transparent flex-1"
                onChange={(e) => state.UpdateLicenseInput(e.target.value)}
                placeholder="Insert License Key"
                value={state.LicenseKey}
              />
              <Button
                className="btn btn-primary btn-sm"
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
