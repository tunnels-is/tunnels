import React from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import dayjs from "dayjs";
import InfoItem from "../components/InfoItem";
import { useNavigate } from "react-router-dom";
import { SquareX } from "lucide-react";
import { useUsers, useDeleteUser } from "@/hooks/useUsers";
import { useSetAtom } from "jotai";
import { userAtom } from "@/stores/userStore";

const UserSelect = () => {
  const navigate = useNavigate();
  const { data: users, isLoading } = useUsers();
  const { mutate: deleteUser } = useDeleteUser();
  const setUser = useSetAtom(userAtom);

  const selectUser = (user) => {
    console.dir(user);
    setUser(user);
    navigate("/account");
  };

  const gotoLogin = () => {
    navigate("/login/1");
  };

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="p-6 space-y-6">
      <Button
        className={"flex items-center gap-1"}
        onClick={() => gotoLogin()}
      >
        {"Add Account"}
      </Button>

      <div className="flex flex-row gap-4">
        {users?.map((u) => (
          <Card
            onClick={() => selectUser(u)}
            key={u._id}
            className="hover:!border-emerald-500 rounded"
          >
            <div className="flex items-center -mt-1 -mr-1">
              <SquareX
                onClick={(e) => {
                  e.stopPropagation();
                  deleteUser(u.SaveFileHash);
                }}
                className="ml-auto text-red"
              />
            </div>
            <CardContent className=" -mt-3 cursor-pointer flex flex-col p-4">
              <InfoItem label="Email" value={u.Email} />
              <InfoItem label="ID" value={u._id} />
              <InfoItem
                label="Server"
                value={u.ControlServer?.Host + ":" + u.ControlServer?.Port}
              />
              <InfoItem
                label="Expiration"
                value={dayjs(u.SubExpiration).format("HH:mm:ss DD-MM-YYYY")}
              />
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
};

export default UserSelect;
