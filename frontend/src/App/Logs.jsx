
import STORE from "@/store";
import GLOBAL_STATE from "../state";
import { useState, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { ArrowLeft, ArrowRight, ChevronsLeft, ChevronsRight } from "lucide-react";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const Logs = () => {
  const state = GLOBAL_STATE("logs")
  const [currentPage, setCurrentPage] = useState(1)
  const [itemsPerPage, setItemsPerPage] = useState(50)

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

  const handleItemsPerPageChange = (newItemsPerPage) => {
    setItemsPerPage(newItemsPerPage)
    setCurrentPage(1) // Reset to first page when changing items per page
  }

  return (
    <div className={classes}>
      
      {/* Pagination Controls */}
      <div className="pagination-controls" style={{ 
        display: 'flex', 
        justifyContent: 'flex-start', 
        alignItems: 'center', 
        padding: '10px 0', 
        borderBottom: '1px solid #333',
        marginBottom: '10px',
        gap: '20px'
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

        <div style={{ display: 'flex', alignItems: 'center', gap: '15px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span>Items per page:</span>
            <Select 
              value={itemsPerPage.toString()} 
              onValueChange={(value) => handleItemsPerPageChange(Number(value))}
            >
              <SelectTrigger className="w-[80px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={"bg-transparent" + state.Theme?.borderColor + state.Theme?.mainBG}>
                <SelectGroup>
                  <SelectItem className={state.Theme?.neutralSelect} value="25">25</SelectItem>
                  <SelectItem className={state.Theme?.neutralSelect} value="50">50</SelectItem>
                  <SelectItem className={state.Theme?.neutralSelect} value="100">100</SelectItem>
                  <SelectItem className={state.Theme?.neutralSelect} value="200">200</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          
          <span>
            Page {currentPage} of {totalPages} ({totalLogs} total logs)
          </span>
        </div>
      </div>

      <div className="logs-window custom-scrollbar">
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
    </div>
  )
}

export default Logs
