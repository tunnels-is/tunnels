import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Edit } from "lucide-react"
import { DeleteIcon } from "lucide-react"
import { useState } from "react"
import { GridLoader } from "react-spinners"


const GenericTable = (props) => {
  const [offset, setOffset] = useState(0)
  const [limit, setLimit] = useState(100)
  const [filter, setFilter] = useState("")
  const [loading, setLoading] = useState(false)

  if (!props.table) {
    return (<></>)
  }

  let t = props.table
  let hdc = "w-[100px] text-sky-300 "
  let ddc = "w-[100px] text-sky-100 "

  const renderHeaders = () => {
    let rows = []
    t.headers?.map(h => {
      let hc = hdc
      if (t.headerClass[h]) {
        hc += t.headerClass[h]()
      }
      rows.push(<TableHead className={hc}>{h}</TableHead>)
    })

    return (
      <TableHeader>
        <TableRow>
          {rows}
        </TableRow>
      </TableHeader>
    )
  }

  const newPage = async (offset, limit) => {
    let shouldLoad = true
    setTimeout(() => {
      if (shouldLoad === true) {
        setLoading(true)
      }
    }, 200)
    await t.more(offset, limit)
    shouldLoad = false
    setLoading(false)
    shouldLoad = false
  }

  const renderRows = () => {
    let rows = []
    let hasFilter = false
    t.data?.forEach((_, i) => {
      hasFilter = false
      let cells = Object.keys(t.columns).map(key => {
        if (t.columns[key] === undefined) {
          return
        }

        let click = t.rowClick ? t.rowClick : () => { }
        if (t.columns[key] !== true) {
          click = t.columns[key]
        }

        let dc = ddc
        if (t.columnClass[key]) {
          dc += + t.columnClass[key]()
        }

        if (t.data[i][key]?.includes && filter !== "") {
          if (t.data[i][key].includes(filter)) {
            hasFilter = true
          }
        } else {
          hasFilter = true
        }

        let cd = t.columFormat[key] ? t.columFormat[key](t.data[i]) : t.data[i][key]
        return <TableCell className={dc} onClick={() => click(t.data[i])} > {cd}</TableCell>
      })

      Object.keys(t.customColumns).forEach(key => {
        cells.push(t.customColumns[key](t.data[i]))
      })

      Object.keys(t.customBtn).forEach(key => {
        cells.push(t.customBtn[key](t.data[i]))
      })

      if (t.Btn?.Edit) {
        cells.push(
          <TableCell className={"w-[10px]"}  >
            <Edit className="h-8 w-8 ml-2" color={"green"} onClick={() => t.Btn.Edit(t.data[i])} />
          </TableCell >
        )
      }

      if (t.Btn?.Delete) {
        cells.push(
          <TableCell className={"w-[10px]"}  >
            <DeleteIcon className="h-8 w-8 ml-2" color={"red"} onClick={() => t.Btn.Delete(t.data[i])} />
          </TableCell >
        )
      }

      if (hasFilter === true) {
        rows.push(
          <TableRow key={i}>
            {cells}
          </TableRow>
        )
      }
    })

    return (
      <TableBody>
        {rows}
      </TableBody>
    )
  }

  return (
    <div className="flex flex-col">
      <div className="flex gap-2 mb-5">
        {t.Btn?.New &&
          <Button onClick={() => t.Btn.New()} > Create</Button>
        }
        <Input
          className="text-white"
          placeholder={"Search.."}
          onChange={(e) => { setFilter(e.target.value) }} />

        <Button onClick={async () => {
          let off = offset - t.opts.RowPerPage
          if (off < 0) {
            off = 0
          }

          setOffset(offset - t.opts.RowPerPage)
          await newPage(offset - t.opts.RowPerPage, t.opts.RowPerPage)
        }}>Prev</Button>

        <Button onClick={async () => {
          let off = offset + t.opts.RowPerPage
          if (off < 0) {
            off = 0
          }
          setOffset(offset + t.opts.RowPerPage)
          await newPage(offset + t.opts.RowPerPage, t.opts.RowPerPage)
        }}>Next</Button>

      </div>

      {
        !loading &&
        <Table>
          {renderHeaders()}
          {renderRows()}
        </Table >
      }

      {
        loading &&
        <GridLoader className={"m-auto"} color={"white"} size={20} />
      }
    </div >
  )
}


export default GenericTable;
