import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useState, useMemo } from "react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { MoreHorizontal, Edit, Trash2, ArrowLeft, ArrowRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";

import { DataTable } from "@/components/DataTable";

const GenericTable = (props) => {
  const [offset, setOffset] = useState(0);
  const [filter, setFilter] = useState("");

  let t = props.table;

  const columns = useMemo(() => {
    if (!t.columns) return [];

    const cols = Object.keys(t.columns).map((key) => {
      if (t.columns[key] === undefined) return null;

      // Header
      let headerText = key;
      if (t.headerFormat && t.headerFormat[key]) {
        headerText = t.headerFormat[key]();
      }

      return {
        accessorKey: key,
        header: headerText,
        meta: {
          className: (original) => {
            let className = "p-2 pl-2 text-white-100 font-medium "; // Adjusted padding
            if (t.columnClass && t.columnClass[key]) {
              className += t.columnClass[key](original);
            }
            return className;
          }
        },
        cell: ({ row, getValue }) => {
          const original = row.original;
          let content = getValue();

          // Format
          if (t.columnFormat && t.columnFormat[key]) {
            content = t.columnFormat[key](original);
          }

          // Click
          let click = t.rowClick ? t.rowClick : () => { };
          if (t.columns[key] !== true) {
            click = t.columns[key];
          }

          // Special handling for Tag, Email, Domain
          if (key === "Tag" || key === "Email" || key === "Domain") {
            return (
              <div onClick={() => click(original)}>
                <Badge variant="secondary" className="cursor-pointer"> {content}</Badge>
              </div>
            );
          }

          return (
            <div onClick={() => click(original)}>
              {content}
            </div>
          );
        }
      };
    }).filter(Boolean);

    // Custom Columns
    if (t.customColumns) {
      Object.keys(t.customColumns).forEach((key) => {
        cols.push({
          id: key,
          header: key,
          cell: ({ row }) => t.customColumns[key](row.original)
        });
      });
    }

    // Actions
    let hasButtons = t.Btn?.Edit || t.Btn?.Delete || t.customBtn;
    if (hasButtons) {
      cols.push({
        id: "actions",
        header: "",
        meta: {
          className: "text-right w-[20px]"
        },
        cell: ({ row }) => {
          const item = row.original;
          const actionItems = [];
          const customButton = [];

          if (t.Btn?.Edit) {
            actionItems.push(
              <DropdownMenuItem
                key="edit"
                onClick={() => t.Btn.Edit(item)}
                className="cursor-pointer"
              >
                <Edit className="w-4 h-4" /> Edit
              </DropdownMenuItem>,
            );
          }
          if (t.Btn?.Delete) {
            actionItems.push(
              <DropdownMenuItem
                key="delete"
                onClick={() => t.Btn.Delete(item)}
                className="cursor-pointer text-red-500"
              >
                <Trash2 className="w-4 h-4" /> Delete
              </DropdownMenuItem>,
            );
          }
          if (t.customBtn) {
            Object.keys(t.customBtn).forEach((key) => {
              const customBtnEl = t.customBtn[key](item);
              customButton.push(customBtnEl);
            });
          }

          return (
            <div className="flex justify-end gap-[5px]">
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" className="h-8 w-8 p-0 text-white">
                    <span className="sr-only">Open menu</span>
                    <MoreHorizontal className="w-4 h-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  {customButton}
                  {actionItems}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          );
        }
      });
    }

    return cols;
  }, [t]);

  const newPage = async (offset, limit) => {
    await t.more(offset, limit);
  };

  return (
    <div className={"flex flex-col gap-5 flex-nowrap " + (props.className ? props.className : "")}>
      <div className="flex flex-col md:flex-row justify-start items-center flex-nowrap">
        <div className="flex gap-2">
          {t.Btn?.Save && (
            <Button
              className="flex items-center gap-1"
              onClick={() => t.Btn.Save()}
            >
              {props.saveButtonLabel ? props.saveButtonLabel : "Save"}
            </Button>
          )}
          {t.Btn?.New && (
            <Button
              className="flex items-center gap-1"
              onClick={() => t.Btn.New()}
            >
              {props.newButtonLabel ? props.newButtonLabel : "Create"}
            </Button>
          )}
          {t.more && (
            <>
              <Button
                variant="outline"
                size="icon"
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
                size="icon"
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

      <DataTable
        columns={columns}
        data={t.data || []}
        pagination={!t.more}
        globalFilter={filter}
        setGlobalFilter={setFilter}
        showSearch={!t.more}
      />
    </div>
  );
};

export default GenericTable;
