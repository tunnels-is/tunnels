import { CloseApp } from "../../wailsjs/go/main/App";
import React from "react";

import { toast } from "react-hot-toast";
import { BarChartIcon, BorderAllIcon, BorderBottomIcon, BorderNoneIcon, CrossCircledIcon, ExitIcon, MinusCircledIcon, PlusCircledIcon, ViewNoneIcon, } from '@radix-ui/react-icons'

import STORE from "../store";

const TopBar = (props) => {

    const TriggerCloseToast = () => {
        toast.error((t) => (
            <div className="exit-confirm">
                <div className="text">
                    Are you sure you want to exit ?
                </div>
                <button className="cancel" onClick={() => toast.dismiss(t.id)}>Cancel</button>
                <button className="exit" onClick={() => HandleClose(t.id)}>Exit</button>
            </div>
        ), { duration: 999999 })
    }

    const HandleClose = (id) => {
        toast.dismiss(id)
        props.toggleLoading({ logTag: "connect", tag: "DISCONNECT", show: true, msg: "Disconnecting and closing the app", includeLogs: true })
        STORE.CleanupOnClose()
        CloseApp()
    }

    return (
        <div className="topbar">
            <div className="drag-area" style={{ "--wails-draggable": "drag" }}></div>

            {/* <BorderNoneIcon className="icon" height={20} width={20} color={"white"} onClick={() => TriggerCloseToast()}></BorderNoneIcon>

            <BorderBottomIcon className="icon" height={20} width={20} color={"white"} onClick={() => Minimize()}></BorderBottomIcon>

            <BorderAllIcon className="icon" height={20} width={20} color={"white"} onClick={() => FullScreen()}></BorderAllIcon> */}
        </div >
    )
}

export default TopBar;
