import React, { useEffect, useState } from "react";
import dayjs from "dayjs";
import GLOBAL_STATE from "../state";
import GenericTable from "./GenericTable";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { useNavigate } from "react-router-dom";

const Groups = () => {
  const state = GLOBAL_STATE("groups");
  const [groups, setGroups] = useState([])
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [group, setGroup] = useState(undefined)
  const navigate = useNavigate()

  let getGroups = async (offset, limit) => {
    let resp = await state.callController(null, null, "POST", "/v3/group/list", {}, false, false)
    if (resp.status === 200) {
      setGroups(resp.data)
    }
  }

  useEffect(() => {
    getGroups(0, 50)
  }, []);

  const saveGroup = async () => {
    let resp = undefined
    let ok = false
    if (group._id !== undefined) {
      resp = await state.callController(null, null, "POST", "/v3/group/update", { Group: group }, false, false)
    } else {
      resp = await state.callController(null, null, "POST", "/v3/group/create", { Group: group }, false, false)
    }

    if (resp.status === 200) {
      ok = true
      if (group._id === undefined) {
        groups.push(resp.data)
        setGroups([...groups])
      }
    } else {
      state.toggleError("unable to create group")
    }
    state.renderPage("groups");
    return ok
  }

  const newGroup = () => {
    setGroup({ Tag: "my-new-group", Description: "This is a new group" })
    setEditModalOpen(true)
    state.renderPage("groups");
  }

  const deleteGroup = async (id) => {
    let resp = await state.callController(null, null, "POST", "/v3/group/delete", { GID: id, }, false, false)
    if (resp.status === 200) {
      let g = groups.filter(g => g._id !== id)
      setGroups([...g])
    }
  }

  let table = {
    data: groups,
    // rowClick: (obj) => {
    //   navigate("/groups/" + obj._id)
    // },
    rowClick: (obj) => {
      console.log("row click!")
      console.dir(obj)
    },
    columns: {
      Tag: (obj) => {
        navigate("/groups/" + obj._id)
      },
      _id: true,
      CreatedAt: true,
    },
    columnFormat: {
      CreatedAt: (obj) => {
        return dayjs(obj.CreatedAt).format("HH:mm:ss DD-MM-YYYY")
      }
    },
    Btn: {
      Edit: (obj) => {
        setGroup(obj)
        setEditModalOpen(true)
      },
      Delete: (obj) => {
        deleteGroup(obj._id)
      },
      New: () => {
        console.log("new!")
        newGroup()
      },
    },
    columnClass: {},
    headers: ["Tag", "ID", "CreatedAt"],
    headerClass: {},
    opts: {
      RowPerPage: 50,
    },
    more: getGroups,
  }

  return (
    <div className="groups-page">
      <GenericTable table={table} />

      <NewObjectEditorDialog
        open={editModalOpen}
        onOpenChange={setEditModalOpen}
        object={group}
        title="Group"
        description=""
        readOnly={false}
        saveButton={async () => {
          let ok = await saveGroup()
          if (ok === true) {
            setEditModalOpen(false)
          }
        }}
        onArrayChange={(key, value, index) => {
          group[key][index] = value;
        }}
        onChange={(key, value, type) => {
          group[key] = value
        }}
      />
    </div >
  )

};

export default Groups;
