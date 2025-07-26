import React from "react";
import GLOBAL_STATE from "../state";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import dayjs from "dayjs";
import InfoItem from "./component/InfoItem";
import { Network } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { useState } from "react";


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
    navigate("/login")
  }

  const loadUsers = async () => {
    await state.GetUsers()
  }

  useEffect(() => {
    loadUsers()
  }, [])

  console.dir(state.Users)

  return (
    <div className="p-6 space-y-6">
      <div className="flex flex-row gap-4">
        {state.Users?.map((u) => (
          <Card
            onClick={() => selectUser(u)}
            key={u._id}
            className="hover:!border-emerald-500 rounded">
            <CardContent className=" cursor-pointer flex flex-col p-4">
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
