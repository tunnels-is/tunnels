import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import GenericTable from "./GenericTable";
import { Button } from "@/components/ui/button";
import InfoItem from "./component/InfoItem";
import { Network } from "lucide-react";
import { Input } from "@/components/ui/input";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { ExitIcon } from "@radix-ui/react-icons";

const Account = () => {
  const navigate = useNavigate();
  const state = GLOBAL_STATE("account");

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

  const LogoutButton = (obj) => {
    return <DropdownMenuItem
      key="connect"
      onClick={() => {
        state.LogoutToken(obj, false);
      }
      }
      className="cursor-pointer text-red-700 focus:text-red-500"
    >
      <ExitIcon className="w-4 h-4 mr-2" /> Logout
    </DropdownMenuItem >

  }

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
      N: (obj) => {
        return "w-[200px]"
      },
      Created: (obj) => {
        console.log("CCCC")
        return "w-[400px]"
      }
    },
    customBtn: {
      Logout: LogoutButton,
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
    <Tabs defaultValue="account">
      <TabsList
        className={state.Theme?.borderColor}
      >
        <TabsTrigger className={state.Theme?.tabs} value="account">Account</TabsTrigger>
        <TabsTrigger className={state.Theme?.tabs} value="loggedin">Devices</TabsTrigger>
        <TabsTrigger className={state.Theme?.tabs} value="license">License Key</TabsTrigger>
      </TabsList>

      <TabsContent key={state?.User?._id} value="account" className="size-fit pl-2 w-[400px]">
        {state?.User && (
          <div className="">
            <div className="space-y-1 text-white">
              <InfoItem
                label="User"
                value={state.User?.Email}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />
              <InfoItem
                label="ID"
                value={state.User?._id}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />
              <InfoItem
                label="Update"
                value={dayjs(state.User?.Updated).format(
                  "DD-MM-YYYY HH:mm:ss",
                )}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />


              {state.User?.SubExpiration && (
                <InfoItem
                  label="Subscription Expires"
                  value={dayjs(state.User?.SubExpiration).format(
                    "DD-MM-YYYY HH:mm:ss",
                  )}
                  icon={< Network className="h-4 w-4 text-blue-400" />}
                />
              )}
              <InfoItem
                label="API Key"
                value={APIKey}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />

              {state.User?.Trial && (
                <InfoItem
                  label="Trial Status"
                  value={state.User?.Trial ? "Active" : "Ended"}
                  icon={<Network className="h-4 w-4 text-blue-400" />}
                />
              )}

            </div>

            <div className="flex flex-col gap-3 mt-6">
              <Button
                variant="outline"
                className={state.Theme?.neutralBtn}
                onClick={() => state.refreshApiKey()}
              >
                Re-Generate API Key
              </Button>

              <Button
                variant="outline"
                className={state.Theme?.neutralBtn}
                onClick={() => navigate("/twofactor/create")}
              >
                Two-Factor Authentication
              </Button>

              <Button
                variant="outline"
                className={state.Theme?.errorBtn}
                onClick={() => state.LogoutAllTokens()}
              >
                Log Out All Devices
              </Button>

            </div>

          </div>
        )}
      </TabsContent>

      <TabsContent value="loggedin" className="size-fit">
        <GenericTable table={table} />
      </TabsContent>
      <TabsContent value="license" className="w-[500px]">
        <KeyValue label="License" value={state.User.Key?.Key} />

        <div className="space-y-3">
          <Input

            onChange={(e) => {
              state.UpdateLicenseInput(e.target.value);
            }}
            name="license"
            placeholder="Insert License Key"
            value={state.LicenseKey}
          />

          <Button
            variant="outline"
            className={state.Theme?.neutralBtn}
            key={state?.LicenseKey}
            onClick={() => state.ActivateLicense()}
          >
            Activate Key
          </Button>
        </div>
      </TabsContent>
    </Tabs>
  );
};

export default Account;
