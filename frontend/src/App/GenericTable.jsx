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
import { useState } from "react";
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

const buttonClass = "font-bold text-white hover:text-black";

const GenericTable = (props) => {
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState(100);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(false);

  if (!props.table) {
    return <></>;
  }

  let t = props.table;
  let hdc = "w-[60px] text-blue-400 font-bold ";
  let ddc = "w-[60px] text-blue-100 font-medium ";

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

    return (
      <TableHeader>
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
            className="cursor-pointer text-red-600 focus:text-red-700"
          >
            <Trash2 className="w-4 h-4 mr-2" /> Delete
          </DropdownMenuItem>,
        );
      }
      const customButton = [];
      if (t.customBtn) {
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
                      className="h-8 w-8 p-0 text-white hover:bg-zinc-800"
                    >
                      <span className="sr-only">Open menu</span>
                      <MoreHorizontal className="w-4 h-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    align="end"
                    className="w-48 bg-zinc-900 text-white border border-zinc-700"
                  >
                    {customButton}
                    {actionItems}
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </div>
          </TableCell>,
        );
      }

      if (hasFilter === true) {
        rows.push(<TableRow key={i}>{cells}</TableRow>);
      }
    });

    return <TableBody>{rows}</TableBody>;
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col md:flex-row justify-between items-center gap-4">
        <div className="flex gap-3 items-center w-full md:w-auto">
          {t.Btn?.New && (
            <Button
              variant="default"
              className="bg-emerald-600 hover:bg-emerald-700 text-white shadow-sm"
              onClick={() => t.Btn.New()}
            >
              <BadgePlus className="w-4 h-4" />
              Create
            </Button>
          )}
          <Input
            className="w-full md:w-64 placeholder:text-muted-foreground"
            placeholder="Search..."
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>

        {t.more && (
          <div className="flex gap-2">
            <Button
              variant="outline"
              className="flex items-center gap-1 text-white"
              onClick={async () => {
                let off = offset - t.opts.RowPerPage;
                if (off < 0) off = 0;
                setOffset(off);
                await newPage(off, t.opts.RowPerPage);
              }}
            >
              <ArrowLeft className="w-4 h-4" />
              Prev
            </Button>

            <Button
              variant="outline"
              className="flex items-center gap-1 text-white"
              onClick={async () => {
                let off = offset + t.opts.RowPerPage;
                if (off < 0) off = 0;
                setOffset(off);
                await newPage(off, t.opts.RowPerPage);
              }}
            >
              Next
              <ArrowRight className="w-4 h-4" />
            </Button>
          </div>
        )}
      </div>

      {!loading && (
        <div className="shadow-sm px-3">
          <Table className="w-full overflow-visible text-sm text-foreground">
            {renderHeaders()}
            {renderRows()}
          </Table>
        </div>
      )}
    </div>
  );
};

export default GenericTable;
