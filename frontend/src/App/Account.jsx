import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";
import NewTable from "./component/newtable";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import GenericTable from "./GenericTable";
import { Button } from "@/components/ui/button";

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



  let APIKey = state.getKey("User", "APIKey");

  let table = {
    data: state.User?.Tokens,
    columns: {
      N: true,
      Created: true,
    },
    columnFormat: {
      Created: (obj) => {
        return dayjs(obj.Created).format("HH:mm:ss DD-MM-YYYY")
      },
      N: (obj) => {
        console.dir(obj)
        if (obj.DT === state?.User?.DeviceToken.DT) {
          return obj.N + " (current)"
        }
        return obj.N
      }
    },
    columnClass: {
      Created: (obj) => {
        console.log("CCCC")
        return "w-[400px]"
      }
    },
    customBtn: {
      Logout: (obj) => {
        return (< Button onClick={() => {
          state.LogoutToken(obj, false);
        }}>
          Logout
        </Button >)
      }
    },
    Btn: {},
    headers: ["N", "Created"],
    headerFormat: {
      N: () => {
        return "Device"
      }
    },
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
  }


  return (
    <div className="account-page p-6">
      <Tabs defaultValue="account">
        <TabsList className="justify-start gap-2 mb-4">
          <TabsTrigger value="account">Account</TabsTrigger>
          <TabsTrigger value="loggedin">Devices</TabsTrigger>
          <TabsTrigger value="license">License Key</TabsTrigger>
        </TabsList>

        <TabsContent key={state?.User?._id} value="account">
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

              </div>

              <div className="flex flex-col gap-3">
                <button
                  className="w-full bg-destructive text-white py-2 rounded-md text-sm font-medium hover:bg-red-600 transition"
                  onClick={() => state.LogoutAllTokens()}
                >
                  Log Out All Devices
                </button>


                <button
                  className="w-full bg-primary text-black py-2 rounded-md text-sm font-medium hover:bg-primary/90 transition"
                  onClick={() => state.refreshApiKey()}
                >
                  Re-Generate API Key
                </button>

                <button
                  className="w-full bg-secondary text-black dark:text-white py-2 rounded-md text-sm font-medium hover:bg-secondary/80 transition"
                  onClick={() => NavigateTo2fa()}
                >
                  Two-Factor Authentication
                </button>
              </div>

            </div>
          )}
        </TabsContent>

        <TabsContent value="loggedin" className=" w-[500px]">
          <GenericTable table={table} />
        </TabsContent>
        <TabsContent value="license" className="border rounded">
          <KeyValue label="License" value={state.User.Key?.Key} />

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
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default Account;
