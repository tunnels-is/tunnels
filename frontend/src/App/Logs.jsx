
import GLOBAL_STATE from "../state";
import WS from "../ws";
import { useState } from "react";

const Logs = () => {
  const state = GLOBAL_STATE("loader")
  const [filter, setFilter] = useState("");
  const [hide, setHide] = useState(false)

  const reloadSocket = () => {
    WS.sockets["logs"] = undefined
    WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent)
  }

  let logs = state.logs
  let classes = "bottom-loader"

  return (
    <div className={classes}  >

      <div className="logs-window custom-scrollbar">
        {logs?.toReversed().map((line, index) => {
          let splitLine = line.split(" || ")
          let error = line.includes("| ERROR |")
          let debug = line.includes("| DEBUG |")
          let info = line.includes("| INFO  |")

          if (filter !== "") {
            if (!line.includes(filter)) {
              return
            }
          }
          return (
            <div className={`line`} key={index}>

              <div className="time">{splitLine[0]}</div>

              {info &&
                <div className="info">{splitLine[1]}</div>
              }
              {error &&
                <div className="error">{splitLine[1]}</div>
              }
              {debug &&
                <div className="debug">{splitLine[1]}</div>
              }
              {!debug && !error && !info &&
                <div className="text"> {splitLine[1]}</div>
              }

              <div className="func">{splitLine[2]}</div>
              <div className="text"> {splitLine.splice(3, 20).join("||")}</div>
            </div >
          )
        })}
      </div>
    </div>
  )
}

export default Logs
