import React from "react";
import GLOBAL_STATE from "../state";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import dayjs from "dayjs";
import InfoItem from "./component/InfoItem";
import { useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { SquareX } from "lucide-react";


const UserSelect = () => {
  const state = GLOBAL_STATE("user-select");
  const navigate = useNavigate();

  const selectUser = (user) => {
    console.dir(user)
    state.SetUser(user)
    navigate("/account")
    window.location.reload()
  }

  const gotoLogin = () => {
    navigate("/login/1")
  }

  const loadUsers = async () => {
    await state.GetUsers()
  }

  useEffect(() => {
    loadUsers()
  }, [])

  console.dir(state.Users)

  return (
    <div className="space-y-4">
      <div className="flex justify-end mb-4">
        <Button
          className={"flex items-center gap-1" + state.Theme?.successBtn}
          onClick={() => gotoLogin()}
        >
          Add Account
        </Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {state.Users?.map((u) => (
          <Card
            onClick={() => selectUser(u)}
            key={u._id}
            className="hover:!border-emerald-500 rounded">
            <div className="flex items-center -mt-1 -mr-1">
              <SquareX
                onClick={() => state.DelUser(u.SaveFileHash)}
                className={'ml-auto' + state.Theme?.redIcon} />
            </div>
            <CardContent className=" -mt-3 cursor-pointer flex flex-col p-4">
              <InfoItem
                label="Email"
                value={u.Email}
              />
              <InfoItem
                label="ID"
                value={u._id}
              />
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


}

export default UserSelect
