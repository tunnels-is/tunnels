import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DeleteIcon } from "lucide-react";
import { useState, useEffect } from "react";
// import { GridLoader } from "react-spinners";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { MoreHorizontal, Edit, Trash2 } from "lucide-react";
import { GridLoader } from "react-spinners";
import { ArrowLeft, ArrowRight, Plus, BadgePlus } from "lucide-react";
import { PlusIcon } from "lucide-react";

import GLOBAL_STATE from "../state";
import { Badge } from "@/components/ui/badge";

const GenericTable = (props) => {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(100);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(false);
  const [isTablet, setIsTablet] = useState(false);
  const state = GLOBAL_STATE("btn+?");

  useEffect(() => {
    const checkTablet = () => {
      setIsTablet(window.innerWidth <= 1024 && window.innerWidth >= 640);
    };
    checkTablet();
    window.addEventListener("resize", checkTablet);
    return () => window.removeEventListener("resize", checkTablet);
  }, []);

  if (!props.table) {
    return <></>;
  }

  let t = props.table;
  let hdc = "w-[60px] text-white font-bold ";
  let ddc = "w-[60px] text-white-100 font-medium ";

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
    let shouldLoad = true;
    setTimeout(() => {
      if (shouldLoad === true) {
        setLoading(true);
      }
    }, 200);
    await t.more(offset, limit);
    shouldLoad = false;
    setLoading(false);
    shouldLoad = false;
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
        if (key === "Tag" || key === "Email") {
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

  const renderCardList = () => {
    return (
      <div className="flex flex-col gap-4">
        {t.data?.map((row, i) => {
          let hasFilter = false;
          let cardContent = Object.keys(t.columns).map((key) => {
            if (t.columns[key] === undefined) return null;
            let click = t.rowClick ? t.rowClick : () => {};
            if (t.columns[key] !== true) click = t.columns[key];
            let dc = ddc;
            if (t.columnClass && t.columnClass[key]) {
              dc += t.columnClass[key](row);
            }
            let cd = row[key];
            if (t.columnFormat && t.columnFormat[key]) {
              cd = t.columnFormat[key](row);
            }
            if (row[key]?.includes && filter !== "") {
              if (row[key].includes(filter)) {
                hasFilter = true;
              }
            } else {
              hasFilter = true;
            }
            if (key === "Tag" || key === "Email") {
              return (
                <div className={"flex items-center gap-2 " + dc} key={key} onClick={() => click(row)}>
                  <span className="font-semibold">{key}:</span>
                  <Badge className={"cursor-pointer" + state.Theme?.badgeNeutral}>{cd}</Badge>
                </div>
              );
            }
            return (
              <div className={"flex items-center gap-2 " + dc} key={key} onClick={() => click(row)}>
                <span className="font-semibold">{key}:</span>
                <span>{cd}</span>
              </div>
            );
          });

          if (t.customColumns) {
            Object.keys(t.customColumns).forEach((key) => {
              cardContent.push(t.customColumns[key](row));
            });
          }

          const actionItems = [];
          let hasButtons = false;
          if (t.Btn?.Edit) {
            hasButtons = true;
            actionItems.push(
              <DropdownMenuItem key="edit" onClick={() => t.Btn.Edit(row)} className="cursor-pointer">
                <Edit className="w-4 h-4 mr-2" /> Edit
              </DropdownMenuItem>
            );
          }
          if (t.Btn?.Delete) {
            hasButtons = true;
            actionItems.push(
              <DropdownMenuItem key="delete" onClick={() => t.Btn.Delete(row)} className="cursor-pointer text-red-500">
                <Trash2 className="w-4 h-4 mr-2" /> Delete
              </DropdownMenuItem>
            );
          }
          const customButton = [];
          if (t.customBtn) {
            hasButtons = true;
            Object.keys(t.customBtn).forEach((key) => {
              const customBtnEl = t.customBtn[key](row);
              customButton.push(customBtnEl);
            });
          }

          if (hasFilter === true) {
            return (
              <div key={i} className="rounded-lg border border-[#1a1f2d] bg-[#0B0E14] p-4 shadow-sm flex flex-col gap-2">
                {cardContent}
                {hasButtons && (
                  <div className="flex justify-end gap-2 mt-2">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" className="h-8 w-8 p-0 text-white">
                          <span className="sr-only">Open menu</span>
                          <MoreHorizontal className="w-4 h-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" className={"w-48 text-white " + state.Theme?.borderColor}>
                        {customButton}
                        {actionItems}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                )}
              </div>
            );
          }
          return null;
        })}
      </div>
    );
  };

  return (
    <div className={"flex flex-col gap-5 " + (props.className ? props.className : "")} >
      <div className="flex flex-col md:flex-row justify-start items-center ">

        {t.more && (
          <div className="flex gap-2">
            <Input
              className="w-full md:w-64 placeholder:text-muted-foreground text-white"
              placeholder="Search..."
              onChange={(e) => setFilter(e.target.value)}
            />
            <Button
              variant="outline"
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
              variant="outline"
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
          </div>
        )}
        {t.Btn?.New && (
          <div className="flex justify-end w-full">
            <Button
              variant="outline"
              className={"flex items-center gap-1" + state.Theme?.successBtn}
              onClick={() => t.Btn.New()}
            >
              {props.newButtonLabel ? props.newButtonLabel : "Create"}
            </Button>
          </div>
        )}
      </div>

      {
        !loading && (
          <div className="shadow-sm">
            {isTablet ? (
              renderCardList()
            ) : (
              <Table className="w-full overflow-visible text-sm text-foreground">
                {renderHeaders()}
                {renderRows()}
              </Table>
            )}
          </div>
        )
      }
    </div >
  );
};

export default GenericTable;
