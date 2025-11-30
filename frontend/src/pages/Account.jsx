import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import dayjs from "dayjs";
import KeyValue from "../components/keyvalue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import GenericTable from "../components/GenericTable";
import { Button } from "@/components/ui/button";
import InfoItem from "../components/InfoItem";
import { Network } from "lucide-react";
import { Input } from "@/components/ui/input";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { ExitIcon } from "@radix-ui/react-icons";
import { useAtomValue, useSetAtom } from "jotai";
import { userAtom } from "../stores/userStore";
import { logout } from "../api/auth";
import { toast } from "sonner";
import { v4 as uuidv4 } from "uuid";
import { useUpdateUser, useActivateLicense } from "../hooks/useAccount";

const Account = () => {
  const navigate = useNavigate();
  const user = useAtomValue(userAtom);
  const setUser = useSetAtom(userAtom);
  const updateUserMutation = useUpdateUser();
  const activateLicenseMutation = useActivateLicense();
  const [licenseKey, setLicenseKey] = useState("");

  const gotoAccountSelect = () => {
    navigate("/accounts");
  }

  if (!user || user.Email === "") {
    gotoAccountSelect()
    return;
  }

  // Sorting tokens
  const sortedTokens = user.Tokens ? [...user.Tokens].sort((x, y) => {
    if (x.Created < y.Created) return 1;
    if (x.Created > y.Created) return -1;
    return 0;
  }) : [];

  let APIKey = user?.APIKey

  const handleLogoutToken = async (token, all) => {
    try {
      await logout({ DeviceToken: token.DT, UserID: user.ID, All: all });
      toast.success("Logged out");
      if (all || token.DT === user.DeviceToken.DT) {
        setUser(null);
        navigate("/login");
      } else {
        const newTokens = user.Tokens.filter(t => t.DT !== token.DT);
        setUser({ ...user, Tokens: newTokens });
      }
    } catch (e) {
      toast.error("Logout failed");
    }
  };

  const LogoutButton = (obj) => {
    return <DropdownMenuItem
      key="connect"
      onClick={() => handleLogoutToken(obj, false)}
      className="cursor-pointer text-red-700 focus:text-red-500"
    >
      <ExitIcon className="w-4 h-4 mr-2" /> Logout
    </DropdownMenuItem >

  }

  const refreshApiKey = async () => {
    const newUser = { ...user, APIKey: uuidv4() };
    try {
      await updateUserMutation.mutateAsync(newUser);
      toast.success("API Key regenerated");
    } catch (e) {
      toast.error("Failed to regenerate API Key");
    }
  };

  const handleActivateLicense = async () => {
    try {
      await activateLicenseMutation.mutateAsync(licenseKey);
      toast.success("License activated");
      setLicenseKey("");
    } catch (e) {
      toast.error("Failed to activate license");
    }
  }

  let table = {
    data: sortedTokens,
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
        if (obj.DT === user?.DeviceToken.DT) {
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
      <TabsList>
        <TabsTrigger value="account">Account</TabsTrigger>
        <TabsTrigger value="loggedin">Devices</TabsTrigger>
        <TabsTrigger value="license">License Key</TabsTrigger>
      </TabsList>

      <TabsContent key={user.ID} value="account" className="size-fit pl-2 w-[400px]">
        {user && (
          <div className="">
            <div className="space-y-1 text-white">
              <InfoItem
                label="User"
                value={user?.Email}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />
              <InfoItem
                label="ID"
                value={user.ID}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />
              <InfoItem
                label="Update"
                value={dayjs(user?.Updated).format(
                  "DD-MM-YYYY HH:mm:ss",
                )}
                icon={<Network className="h-4 w-4 text-blue-400" />}
              />


              {user?.SubExpiration && (
                <InfoItem
                  label="Subscription Expires"
                  value={dayjs(user?.SubExpiration).format(
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

              {user?.Trial && (
                <InfoItem
                  label="Trial Status"
                  value={user?.Trial ? "Active" : "Ended"}
                  icon={<Network className="h-4 w-4 text-blue-400" />}
                />
              )}

            </div>

            <div className="flex flex-col gap-3 mt-6">
              <Button
                onClick={() => gotoAccountSelect()}
              >
                Switch Account
              </Button>
              <Button
                onClick={refreshApiKey}
              >
                Re-Generate API Key
              </Button>

              <Button
                onClick={() => navigate("/twofactor/create")}
              >
                Two-Factor Authentication
              </Button>

              <Button
                onClick={() => handleLogoutToken(user.DeviceToken, true)}
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
        <KeyValue label="License" value={user.Key?.Key} />

        <div className="space-y-3">
          <Input
            onChange={(e) => setLicenseKey(e.target.value)}
            name="license"
            placeholder="Insert License Key"
            value={licenseKey}
          />

          <Button
            onClick={handleActivateLicense}
          >
            Activate Key
          </Button>
        </div>
      </TabsContent>
    </Tabs>
  );
};

export default Account;
