
import STORE from "@/store";
import GLOBAL_STATE from "../state";
import { useState, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowLeft, ArrowRight, ChevronsLeft, ChevronsRight } from "lucide-react";

const Logs = () => {
  const state = GLOBAL_STATE("logs")
  const [currentPage, setCurrentPage] = useState(1)
  const [searchFilter, setSearchFilter] = useState("")
  const [tagFilter, setTagFilter] = useState("")
  const itemsPerPage = 100

  let logs = STORE.Cache.GetObject("logs")
  let classes = "logs-loader"

  // Filter logs based on search and tag filter
  const filteredLogs = useMemo(() => {
    if (!logs) return []
    let filtered = logs

    // Apply search filter
    if (searchFilter) {
      filtered = filtered.filter(line =>
        line.toLowerCase().includes(searchFilter.toLowerCase())
      )
    }

    // Apply tag filter
    if (tagFilter) {
      filtered = filtered.filter(line =>
        line.includes(`| ${tagFilter} |`)
      )
    }

    return filtered
  }, [logs, searchFilter, tagFilter])

  // Calculate pagination on filtered logs
  const totalLogs = filteredLogs?.length || 0
  const totalPages = Math.ceil(totalLogs / itemsPerPage)

  // Get current page logs from filtered results
  const paginatedLogs = useMemo(() => {
    if (!filteredLogs) return []
    const reversedLogs = filteredLogs.toReversed()
    const startIndex = (currentPage - 1) * itemsPerPage
    const endIndex = startIndex + itemsPerPage
    return reversedLogs.slice(startIndex, endIndex)
  }, [filteredLogs, currentPage, itemsPerPage])

  const goToPage = (page) => {
    setCurrentPage(Math.max(1, Math.min(page, totalPages)))
  }

  // Reset to page 1 when search filter changes
  const handleSearchChange = (e) => {
    setSearchFilter(e.target.value)
    setCurrentPage(1)
  }

  // Reset to page 1 when tag filter changes
  const handleTagFilterChange = (value) => {
    // Convert "all" back to empty string for filtering logic
    setTagFilter(value === "all" ? "" : value)
    setCurrentPage(1)
  }

  return (
    <div className={classes} style={{
      display: 'flex', flexDirection: 'column'
    }}>

      {/* Pagination Controls */}
      < div className="pagination-controls bg-black z-[1000] pb-[15px] pt-[20px] fixed top-[0px]" >
        <div className="flex gap-[10px]" >
          <Select
            value={tagFilter || "all"}
            onValueChange={handleTagFilterChange}
          >
            <SelectTrigger className="w-[120px] text-white">
              <SelectValue placeholder="Tag filter" />
            </SelectTrigger>
            <SelectContent className={"bg-transparent" + state.Theme?.borderColor + state.Theme?.mainBG}>
              <SelectGroup>
                <SelectItem className={state.Theme?.neutralSelect} value="all">All</SelectItem>
                <SelectItem className={state.Theme?.neutralSelect} value="INFO">INFO</SelectItem>
                <SelectItem className={state.Theme?.neutralSelect} value="ERROR">ERROR</SelectItem>
                <SelectItem className={state.Theme?.neutralSelect} value="DEBUG">DEBUG</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
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


          <Input
            className="w-full md:w-64 placeholder:text-muted-foreground text-white"
            placeholder="Search logs..."
            value={searchFilter}
            onChange={handleSearchChange}
          />
        </div>

      </div >

      <div className="logs-window custom-scrollbar flex flex-col mt-[50px] overflow-auto">
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
