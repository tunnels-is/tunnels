import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useState } from "react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { MoreHorizontal, Edit, Trash2 } from "lucide-react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import GLOBAL_STATE from "../state";
import { Badge } from "@/components/ui/badge";

const GenericTable = (props) => {
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState("");
  const state = GLOBAL_STATE("btn+?");

  let t = props.table;
  let hdc = "w-[60px] text-white font-bold ";
  let ddc = "w-[60px] h-[30px] p-0 pl-2 text-white-100 font-medium ";

  const renderHeaders = () => {
    let rows = [];
    t.headers?.map((h) => {
      let hc = hdc;
      if (t.headerClass && t.headerClass[h]) {
        hc += t.headerClass[h]();
      }
      let out = h;
      if (t.headerFormat && t.headerFormat[h]) {
        out = t.headerFormat[h]();
      }
      rows.push(<TableHead className={hc}>{out}</TableHead>);
    });

    let hasButtons = false
    if (t.Btn?.Edit) { hasButtons = true }
    if (t.Btn?.Delete) { hasButtons = true }
    if (t.customBtn) {
      hasButtons = true
    }
    if (hasButtons) {
      rows.push(<TableHead className={"btnx"}></TableHead>);
    }

    return (
      <TableHeader className="bg-[#0B0E14] border border-[#1a1f2d] rounded-full">
        <TableRow>{rows}</TableRow>
      </TableHeader>
    );
  };

  const newPage = async (offset, limit) => {
    // let shouldLoad = true;
    // setTimeout(() => {
    //   if (shouldLoad === true) {
    //     setLoading(true);
    //   }
    // }, 200);
    await t.more(offset, limit);
    // shouldLoad = false;
    // setLoading(false);
    // shouldLoad = false;
  };

  const renderRows = () => {
    let rows = [];
    let hasFilter = false;
    t.data?.forEach((_, i) => {
      hasFilter = false;
      let cells = Object.keys(t.columns).map((key) => {
        if (t.columns[key] === undefined) {
          return;
        }

        let click = t.rowClick ? t.rowClick : () => { };
        if (t.columns[key] !== true) {
          click = t.columns[key];
        }

        let dc = ddc;
        if (t.columnClass && t.columnClass[key]) {
          dc += t.columnClass[key](t.data[i]);
        }

        if (t.data[i][key]?.includes && filter !== "") {
          if (t.data[i][key].includes(filter)) {
            hasFilter = true;
          }
        } else {
          hasFilter = true;
        }

        let cd = t.data[i][key];
        if (t.columnFormat && t.columnFormat[key]) {
          cd = t.columnFormat[key](t.data[i]);
        }
        if (key === "Tag" || key === "Email" || key === "Domain") {
          return (
            <TableCell className={dc} onClick={() => click(t.data[i])}>
              <Badge className={"cursor-pointer" + state.Theme?.badgeNeutral}> {cd}</Badge>
            </TableCell>
          );

        }
        return (
          <TableCell className={dc} onClick={() => click(t.data[i])}>
            {cd}
          </TableCell>
        );
      });

      if (t.customColumns) {
        Object.keys(t.customColumns).forEach((key) => {
          cells.push(t.customColumns[key](t.data[i]));
        });
      }

      const actionItems = [];

      let hasButtons = false
      if (t.Btn?.Edit) {
        hasButtons = true
        actionItems.push(
          <DropdownMenuItem
            key="edit"
            onClick={() => t.Btn.Edit(t.data[i])}
            className="cursor-pointer"
          >
            <Edit className="w-4 h-4 mr-2" /> Edit
          </DropdownMenuItem>,
        );
      }

      if (t.Btn?.Delete) {
        hasButtons = true
        actionItems.push(
          <DropdownMenuItem
            key="delete"
            onClick={() => t.Btn.Delete(t.data[i])}
            className="cursor-pointer text-red-500"
          >
            <Trash2 className="w-4 h-4 mr-2" /> Delete
          </DropdownMenuItem>,
        );
      }
      const customButton = [];
      if (t.customBtn) {
        hasButtons = true
        Object.keys(t.customBtn).forEach((key) => {
          const customBtnEl = t.customBtn[key](t.data[i]);
          customButton.push(customBtnEl);
        });
      }

      if (hasButtons === true) {
        cells.push(
          <TableCell className="text-right w-[20px]">
            <div className="flex justify-end gap-[5px]">
              {hasButtons && (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      className="h-8 w-8 p-0 text-white"
                    >
                      <span className="sr-only">Open menu</span>
                      <MoreHorizontal className="w-4 h-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    className={"w-48 text-white " + state.Theme?.borderColor}
                  >
                    {customButton}
                    {actionItems}
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
          </TableCell >,
        );
      }

      if (hasFilter === true) {
        rows.push(<TableRow key={i}>{cells}</TableRow>);
      }
    });

    return <TableBody>{rows}</TableBody>;
  };

  return (
    <div className={"flex flex-col gap-5  flex-nowrap" + (props.className ? props.className : "")} >
      <div className="flex flex-col md:flex-row justify-start items-center  flex-nowrap">

        <div className="flex gap-2">
          {t.Btn?.Save && (
            <div className="flex mr-2 ">
              <Button
                className={"flex  items-center gap-1" + state.Theme?.successBtn}
                onClick={() => t.Btn.Save()}
              >
                {props.saveButtonLabel ? props.saveButtonLabel : "Save"}
              </Button>
            </div>
          )}
          {t.Btn?.New && (
            <div className="flex mr-2">
              <Button
                className={"flex items-center gap-1" + state.Theme?.successBtn}
                onClick={() => t.Btn.New()}
              >
                {props.newButtonLabel ? props.newButtonLabel : "Create"}
              </Button>
            </div>
          )}
          {t.more && (
            <>
              <Button
                className={"flex items-center gap-1" + state.Theme?.neutralBtn}
                onClick={async () => {
                  let off = offset - t.opts.RowPerPage;
                  if (off < 0) off = 0;
                  setOffset(off);
                  await newPage(off, t.opts.RowPerPage);
                }}
              >
                <ArrowLeft className="w-4 h-4" />
              </Button>

              <Button
                className={"flex items-center gap-1" + state.Theme?.neutralBtn}
                onClick={async () => {
                  let off = offset + t.opts.RowPerPage;
                  if (off < 0) off = 0;
                  setOffset(off);
                  await newPage(off, t.opts.RowPerPage);
                }}
              >
                <ArrowRight className="w-4 h-4" />
              </Button>
              <Input
                className="w-full md:w-64 placeholder:text-muted-foreground text-white"
                placeholder="Search..."
                onChange={(e) => setFilter(e.target.value)}
              />
            </>
          )}
        </div>

      </div>

      <Table className="w-full overflow-visible text-sm text-foreground">
        {renderHeaders()}
        <div class="h-2"></div>
        {renderRows()}
      </Table>
    </div >
  );
};

export default GenericTable;
