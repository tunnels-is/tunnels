import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";
import NewTable from "./component/newtable";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const Account = () => {
  const navigate = useNavigate();
  const state = GLOBAL_STATE("account");

  const NavigateTo2fa = () => {
    navigate("/twofactor");
  };

  if (state.User?.Email === "" || !state.User) {
    navigate("/login");
    return;
  }

  useEffect(() => {
    state.GetBackendState();
  }, []);

  state.User?.Tokens?.sort(function(x, y) {
    if (x.Created < y.Created) {
      return 1;
    }
    if (x.Created > y.Created) {
      return -1;
    }
    return 0;
  });

  const generateListTable = (tokens) => {
    let rows = [];
    tokens?.forEach((token) => {
      let current = false;
      if (token.DT === state?.User?.DeviceToken.DT) {
        current = true;
      }

      let row = {};
      row.items = [
        { type: "text", value: current ? token.N + " (this device)" : token.N },
        {
          type: "text",
          value: dayjs(token.Created).format("DD-MM-YYYY HH:mm:ss"),
        },
        {
          type: "text",
          click: () => {
            state.LogoutToken(token, false);
          },
          value: (
            <div className={`logout clickable`} value={"Logout"}>
              Logout
            </div>
          ),
        },
      ];
      rows.push(row);
    });
    return rows;
  };

  let rows = generateListTable(state.User?.Tokens);
  const headers = [{ value: "Device" }, { value: "Login Date" }, { value: "" }];

  let APIKey = state.getKey("User", "APIKey");

  return (
    <div className="account-page p-6 max-w-xl">
      <Tabs defaultValue="account">
        <TabsList className=" justify-start gap-2 mb-4">
          <TabsTrigger value="account">Account</TabsTrigger>
          <TabsTrigger value="loggedin">Logged In Devices</TabsTrigger>
        </TabsList>

        <TabsContent value="account">
          {state?.User && (
            <div className="space-y-6 rounded-xl border p-6 shadow-sm bg-black">
              <div className="space-y-4">
                <KeyValue label="User" value={state.User?.Email} />
                <KeyValue
                  label="Last Update"
                  value={dayjs(state.User.Updated).format(
                    "DD-MM-YYYY HH:mm:ss",
                  )}
                />
                <KeyValue label="ID" value={state.User._id} />
                <KeyValue
                  label="API Key"
                  defaultValue="not set.."
                  value={APIKey}
                />

                {state.User.SubExpiration && (
                  <KeyValue
                    label="Subscription Expires"
                    value={dayjs(state.User.SubExpiration).format(
                      "DD-MM-YYYY HH:mm:ss",
                    )}
                  />
                )}

                {state.User.Trial && (
                  <KeyValue
                    label="Trial Status"
                    value={state.User.Trial ? "Active" : "Ended"}
                  />
                )}

                <KeyValue label="License" value={state.User.Key?.Key} />
              </div>

              <div className="flex flex-col gap-3">
                <button
                  className="w-full bg-destructive text-white py-2 rounded-md text-sm font-medium hover:bg-red-600 transition"
                  onClick={() => state.LogoutAllTokens()}
                >
                  Log Out All Devices
                </button>

                {!state.modifiedUser ? (
                  <button
                    className="w-full bg-primary text-black py-2 rounded-md text-sm font-medium hover:bg-primary/90 transition"
                    onClick={() => state.refreshApiKey()}
                  >
                    Re-Generate API Key
                  </button>
                ) : (
                  <button
                    className="w-full bg-primary text-white py-2 rounded-md text-sm font-medium hover:bg-primary/90 transition"
                    onClick={() => state.UpdateUser()}
                  >
                    Save API Key
                  </button>
                )}

                <button
                  className="w-full bg-secondary text-black dark:text-white py-2 rounded-md text-sm font-medium hover:bg-secondary/80 transition"
                  onClick={() => NavigateTo2fa()}
                >
                  Two-Factor Authentication
                </button>
              </div>

              <div className="space-y-3">
                <input
                  onChange={(e) => {
                    state.UpdateLicenseInput(e.target.value);
                  }}
                  name="license"
                  className="w-full px-4 py-2 rounded-md border text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                  placeholder="Insert License Key"
                  value={state.LicenseKey}
                />

                <button
                  key={state?.LicenseKey}
                  className="w-full bg-primary text-black py-2 rounded-md text-sm font-medium hover:bg-primary/90 transition"
                  onClick={() => state.ActivateLicense()}
                >
                  Activate Key
                </button>
              </div>
            </div>
          )}
        </TabsContent>

        <TabsContent value="loggedin" className="border rounded">
          <NewTable
            tableID="devices"
            title="Logged In Devices"
            className="logins-list-table"
            background={true}
            header={headers}
            rows={rows}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default Account;
