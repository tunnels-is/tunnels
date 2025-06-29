
import STORE from "@/store";
import GLOBAL_STATE from "../state";
import { useState, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ArrowLeft, ArrowRight, ChevronsLeft, ChevronsRight } from "lucide-react";

const Logs = () => {
  const state = GLOBAL_STATE("logs")
  const [currentPage, setCurrentPage] = useState(1)
  const itemsPerPage = 100

  let logs = STORE.Cache.GetObject("logs")
  let classes = "logs-loader"

  // Calculate pagination
  const totalLogs = logs?.length || 0
  const totalPages = Math.ceil(totalLogs / itemsPerPage)

  // Get current page logs
  const paginatedLogs = useMemo(() => {
    if (!logs) return []
    const reversedLogs = logs.toReversed()
    const startIndex = (currentPage - 1) * itemsPerPage
    const endIndex = startIndex + itemsPerPage
    return reversedLogs.slice(startIndex, endIndex)
  }, [logs, currentPage, itemsPerPage])

  const goToPage = (page) => {
    setCurrentPage(Math.max(1, Math.min(page, totalPages)))
  }

  return (
    <div className={classes} style={{
      display: 'flex', flexDirection: 'column'
    }}>

      {/* Pagination Controls */}
      < div className="pagination-controls" style={{
        display: 'flex',
        justifyContent: 'flex-start',
        alignItems: 'center',
        padding: '10px 0',
        borderBottom: '1px solid #333',
        marginBottom: '10px',
        gap: '20px',
        flexShrink: 0
      }}>
        <div style={{ display: 'flex', gap: '8px' }}>
          <Button
            className={"flex items-center gap-1" + state.Theme?.neutralBtn}
            onClick={() => goToPage(1)}
            disabled={currentPage === 1}
          >
            <ChevronsLeft className="w-4 h-4" />
          </Button>
          <Button
            className={"flex items-center gap-1" + state.Theme?.neutralBtn}
            onClick={() => goToPage(currentPage - 1)}
            disabled={currentPage === 1}
          >
            <ArrowLeft className="w-4 h-4" />
          </Button>
          <Button
            className={"flex items-center gap-1" + state.Theme?.neutralBtn}
            onClick={() => goToPage(currentPage + 1)}
            disabled={currentPage === totalPages}
          >
            <ArrowRight className="w-4 h-4" />
          </Button>
          <Button
            className={"flex items-center gap-1" + state.Theme?.neutralBtn}
            onClick={() => goToPage(totalPages)}
            disabled={currentPage === totalPages}
          >
            <ChevronsRight className="w-4 h-4" />
          </Button>
        </div>

        <span>
          Page {currentPage} of {totalPages} ({totalLogs} total logs, showing {itemsPerPage} per page)
        </span>
      </div >

      <div className="logs-window custom-scrollbar" style={{ flex: 1, overflow: 'auto' }}>
        {paginatedLogs?.map((line, index) => {
          let splitLine = line.split(" || ")
          let error = line.includes("| ERROR |")
          let debug = line.includes("| DEBUG |")
          let info = line.includes("| INFO  |")

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
    </div >
  )
}

export default Logs
