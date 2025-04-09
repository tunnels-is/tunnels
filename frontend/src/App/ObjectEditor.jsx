import { useEffect, useState } from "react";
import React from "react";
import GLOBAL_STATE from "../state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Plus, X, Save, ArrowLeft, Trash2 } from "lucide-react";

const ObjectEditor = (props) => {
  const state = GLOBAL_STATE("oe");
  const [trigger, setTrigger] = useState({ id: 1 });

  const reload = () => {
    let xx = { ...trigger };
    xx.id += 1;
    setTrigger(xx);
  };

  const noItems = React.createElement(
    "div",
    {
      key: "no-items",
      className: "no-items text-gray-400 p-4 text-center text-sm",
    },
    "No items available",
  );

  const transformType = (t) => {
    if (t === "boolean") {
      return "checkbox";
    } else if (t === "string") {
      return "text";
    }
    return t;
  };

  const makeX = (data, type, ns, id, parent, opts) => {
    if (data === undefined || data === null) {
      return { type: type, id: id, ns: ns, parent: parent, origin: data };
    }
    let d = String(opts.meta.depth);
    let x = {
      id: id,
      ns: String(ns),
      parent: parent,
      type: type,
      key: ns + id + d + opts.meta.index,
      ds: "d" + d,
      className: "d" + d + " " + id + " " + ns,
      extraClasses: "",
      delButton: undefined,
      newButton: undefined,
      nested: [],
      origin: data,
      title: "",
    };

    if (opts.meta.index !== undefined) {
      x.index = String(opts.meta.index);
      x.className = x.className + "_" + x.index;
    }

    if (x.type === "array") {
      if (!containsObj(data)) {
        x.className = x.className + " arr_grp_column";
      }
    }
    if (type === "object") {
      x.className += " obj_grp";
    } else if (type === "array") {
      x.className += " arr_grp";
    }

    if (x.title === "") {
      if (x.type === "array") {
        x.title = x.id;
      } else if (x.type !== "object") {
        x.title = x.id;
      }
    }

    if (props.opts.titles && props.opts.titles[x.ns] !== undefined) {
      if (x.index !== undefined && x.index !== "") {
        x.title = props.opts.titles[x.ns + "_" + x.index];
      } else {
        x.title = props.opts.titles[x.ns];
      }
    }

    if (x.title === undefined) {
      x.title = data["Tag"];
    }
    if (x.title === undefined) {
      x.title = data["Name"];
    }
    if (x.title === undefined) {
      x.title = data["Title"];
    }

    if (opts.delButtons !== undefined) {
      x.delButton = opts.delButtons[ns];
    }
    if (x.index === undefined) {
      if (opts.newButtons !== undefined) {
        x.newButton = opts.newButtons[ns];
      }
    }

    if (parent.type === "array" && x.delButton === undefined) {
      x.delButton = () => {
        parent.origin.splice(id, 1);
        reload();
      };
    }
    return x;
  };

  const getType = (data) => {
    switch (Object.prototype.toString.call(data)) {
      case "[object Array]":
        return "array";
      case "[object Object]":
        return "object";
      default:
        let to = typeof data;
        switch (to) {
          case "boolean":
            return "boolean";
          case "number":
            return "number";
          default:
            return "string";
        }
    }
  };

  const containsObj = (data) => {
    let hasObjects = false;
    switch (Object.prototype.toString.call(data)) {
      case "[object Object]":
        Object.keys(data).forEach((v) => {
          switch (Object.prototype.toString.call(data[v])) {
            case "[object Array]":
              hasObjects = true;
              return;
            case "[object Object]":
              hasObjects = true;
              return;
          }
        });
        break;
      case "[object Array]":
        data.forEach((v) => {
          switch (Object.prototype.toString.call(v)) {
            case "[object Array]":
              hasObjects = true;
              return;
            case "[object Object]":
              hasObjects = true;
              return;
          }
        });
        break;
    }
    return hasObjects;
  };

  const switchX = (data, id, ns, parent, opts) => {
    switch (getType(data)) {
      case "array":
        walkA(data, id, ns, parent, opts);
        break;
      case "object":
        walkO(data, id, ns, parent, opts);
        break;
      default:
        let to = typeof data;
        ns = ns + "_" + id;
        let x = makeX(data, to, ns, id, parent, opts);
        parent.nested.push(x);
    }
    return;
  };

  const walkO = (data, id, ns, parent, opts) => {
    opts.meta.depth += 1;
    if (parent.type === "array") {
      ns = ns ? ns : "";
    } else {
      ns = (ns ? ns + "_" : "") + id;
    }
    let x = makeX(data, "object", ns, id, parent, opts);
    parent.nested.push(x);

    let objKeys = [];
    let otherKeys = [];
    let boolKeys = [];
    Object.keys(data).forEach((v) => {
      if (typeof data[v] === "object") {
        objKeys.push(v);
      } else if (typeof data[v] === "boolean") {
        boolKeys.push(v);
      } else {
        otherKeys.push(v);
      }
    });

    otherKeys.forEach((k) => {
      opts.meta.index = undefined;
      switchX(data[k], k, ns, x, opts);
    });

    boolKeys.forEach((k) => {
      opts.meta.index = undefined;
      switchX(data[k], k, ns, x, opts);
    });

    objKeys.forEach((k) => {
      opts.meta.index = undefined;
      switchX(data[k], k, ns, x, opts);
    });

    opts.meta.depth -= 1;
  };

  const walkA = (data, id, ns, parent, opts) => {
    opts.meta.depth += 1;
    ns = (ns ? ns + "_" : "") + id;
    let x = makeX(data, "array", ns, id, parent, opts);
    parent.nested.push(x);

    opts.meta.index = undefined;
    data.forEach((v, i) => {
      opts.meta.index = i;
      switchX(v, i, ns, x, opts);
    });
    opts.meta.index = undefined;

    opts.meta.depth -= 1;
  };

  const makeMeta = (data, opts) => {
    let bt = getType(data);
    opts.meta = {
      index: undefined,
      parent: {},
      depth: 0,
      rootKeys: [],
      rootArrays: [],
      rootObjects: [],
      nested: [],
      type: bt,
      origin: data,
      id: "root",
      ns: "root",
    };

    // console.log("basetype", bt)
    if (bt === "object") {
      Object.keys(data).forEach((k) => {
        if (
          (data[k] === null || data[k] === undefined) &&
          props.opts.defaults
        ) {
          let def = props.opts.defaults["root_" + k];
          if (def) {
            data[k] = def;
          }
        }
        let gt = getType(data[k]);

        // TODO .. ignore undefined, null and functions
        if (gt !== "object" && gt !== "array") {
          let x = makeX(data[k], gt, "root_" + k, k, opts.meta, opts);
          opts.meta.rootKeys.push(x);
        } else {
          if (containsObj(data[k]) === false) {
            let x = makeX(data[k], gt, "root_" + k, k, opts.meta, opts);
            if (gt === "array") {
              opts.meta.index = undefined;
              data[k].forEach((v, i) => {
                opts.meta.index = i;
                switchX(v, i, "root", x, opts);
              });
              opts.meta.index = undefined;
              opts.meta.rootArrays.push(x);
            } else if (gt === "object") {
              Object.keys(data[k]).forEach((v) => {
                opts.meta.index = undefined;
                switchX(data[k][v], v, "root", x, opts);
              });
              opts.meta.rootObjects.push(x);
            }
          } else {
            if (gt === "array") {
              // console.log("KEY:", k)
              opts.meta.index = undefined;
              walkA(data[k], k, "root", opts.meta, opts);
            } else if (gt === "object") {
              opts.meta.index = undefined;
              walkO(data[k], k, "root", opts.meta, opts);
            }
          }
        }
      });

      // do main walk
      // 		walkO(data, "root", data, "", root, opts)
    } else if (bt === "array") {
      data.forEach((v, i) => {
        let gt = getType(v);
        if (gt !== "object" && gt !== "array") {
          let x = makeX(v, gt, "root", i, opts.meta, opts);
          opts.meta.rootKeys.push(x);
        } else {
          if (containsObj(v) === false) {
            let x = makeX(v, gt, "root", i, opts.meta, opts);
            if (gt === "array") {
              opts.meta.index = undefined;
              v.forEach((vv, i) => {
                opts.meta.index = i;
                switchX(vv, i, "root", x, opts);
              });
              opts.meta.index = undefined;
              opts.meta.rootArrays.push(x);
            } else if (gt === "object") {
              Object.keys(v).forEach((vv) => {
                opts.meta.index = undefined;
                switchX(v[vv], vv, "root", x, opts);
              });
              opts.meta.rootObjects.push(x);
            }
          } else {
            if (gt === "array") {
              opts.meta.index = undefined;
              walkA(v, i, "root", opts.meta, opts);
            } else if (gt === "object") {
              opts.meta.index = undefined;
              walkO(v, i, "root", opts.meta, opts);
            }
          }
        }
      });
    } else {
      // ????
    }

    // console.log("META")
    // console.dir(opts.meta)
  };

  const makeInput = (x) => {
    if (props.opts.hidden && props.opts.hidden["root_" + x.id] === true) {
      return;
    }
    let input = null;
    let label = null;
    let checked = undefined;
    if (x.type === "boolean") {
      checked = x.parent.origin[x.id];
      input = React.createElement(Checkbox, {
        key: x.key + "_checkbox",
        className: checked ? "accent-emerald-500 cursor-pointer" : "accent-emerald-500 cursor-pointer",
        checked: x.parent.origin[x.id],
        onCheckedChange: (checked) => {
          x.parent.origin[x.id] = checked;
          state.renderPage("oe");
        },
      });
    } else {
      let disabled = false;
      if (props.opts.disabled && props.opts.disabled["root_" + x.id] === true) {
        disabled = true;
      } else if (props.opts.readOnly === true) {
        disabled = true;
      }
      input = React.createElement(Input, {
        key: x.key + "_input",
        className: "h-9 px-3 py-1.5 bg-[#0f0f0f] border-[#333] rounded-md text-white/90 text-sm focus:ring-blue-500 focus:border-blue-500 w-full",
        value: x.parent.origin[x.id],
        type: transformType(x.type),
        disabled: disabled,
        onChange: (e) => {
          if (x.type === "number") {
            x.parent.origin[x.id] = Number(e.target.value);
          } else {
            x.parent.origin[x.id] = String(e.target.value);
          }
          state.renderPage("oe");
        },
      });
    }

    if (x.parent.type === "array") {
      label = React.createElement(
        Button,
        {
          key: x.key + "_rem_button",
          variant: "outline",
          size: "icon",
          className: "h-6 w-6 rounded-full bg-red-500/10 text-red-400 hover:bg-red-500/20 border-transparent p-0",
          onClick: () => {
            x.parent.origin.splice(x.id, 1);
            state.renderPage("oe");
          },
        },
        React.createElement(X, { className: "h-3 w-3" })
      );
    } else {
      label = React.createElement(
        "div",
        {
          key: x.key + "_label",
          className: checked ? "label checked text-sm font-medium text-emerald-400" : "label text-sm font-medium text-white/80",
        },
        x.title,
      );
    }

    if (x.type === "boolean") {
      return React.createElement(
        "div",
        {
          key: x.key + "_input_wrap",
          className:
            x.id +
            " input_wrap flex items-center gap-2 p-2 " +
            (x.type !== "boolean" ? "bottom_border border-b border-[#222] mb-2" : "") +
            " " +
            x.ns,
        },
        input,
        label,
      );
    }

    return React.createElement(
      "div",
      {
        key: x.key + "_input_wrap",
        className:
          x.id +
          " input_wrap flex flex-col gap-1.5 mb-3 " +
          (x.type !== "boolean" ? "" : "") +
          " " +
          x.ns,
      },
      label,
      input,
    );
  };

  const walkNested = (x) => {
    let sub = [];
    if (x.type === "object" || x.type === "array") {
      if (x.nested?.length < 1) {
        sub.push(noItems);
      } else {
        x.nested?.map((xx) => {
          sub.push(walkNested(xx));
        });
      }
    } else {
      return makeInput(x);
    }

    if (props.opts.hidden && props.opts.hidden["root_" + x.id] === true) {
      return;
    }

    let titleD = null;
    let newB = null;
    let delB = null;
    let topB = null;

    if (x.title !== "" && x.title !== undefined){
      
    titleD = React.createElement(
            "div",
            {
              key: x.key + "_title",
              className: "editor-title",
            },
            x.title,
          )
    }

    if (titleD === null && x.type === "array") {
      titleD =
        x.id !== ""
          ? React.createElement(
              "div",
              {
                key: x.key + "_title",
                className: "editor-title text-base font-medium text-white/90 mb-2",
              },
              x.id,
            )
          : null;
    }

    newB =
      x.newButton !== undefined
        ? React.createElement(
            Button,
            {
              key: x.key + "_new_button",
              variant: "outline",
              size: "icon",
              className: "h-6 w-6 p-0 rounded-full bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 border-transparent",
              onClick: () => {
                x.newButton(x.origin);
                reload();
              },
            },
            React.createElement(Plus, { className: "h-3 w-3" })
          )
        : null;

    delB =
      x.delButton !== undefined
        ? React.createElement(
            Button,
            {
              key: x.key + "_del_button",
              variant: "outline",
              size: "sm",
              className: "h-7 px-2 py-1 text-xs bg-red-500/10 text-red-400 hover:bg-red-500/20 border-transparent",
              onClick: () => {
                x.delButton(x.parent.origin);
                reload();
              },
            },
            React.createElement(Trash2, { className: "h-3 w-3 mr-1" }),
            x.type === "object" ? "Delete" : "Delete"
          )
        : null;

    if (titleD !== null || newB !== null) {
      topB = React.createElement(
        "div",
        {
          key: x.key + "_top_bar",
          className: "top_bar flex items-center justify-between mb-2 mt-1",
        },
        titleD,
        newB,
      );
    }

    return React.createElement(
      "div",
      {
        key: x.key,
        className: x.className + " " + x.extraClasses + " p-3 bg-[#0d0d0d] rounded-md mb-3 border border-[#222] shadow-sm",
      },
      delB,
      topB,
      ...sub,
    );
  };

  const makeDom = (opts) => {
    let rootKeys = [];
    opts.meta.rootKeys.map((k, i) => {
      if (k.type !== "boolean") {
        rootKeys.push(makeInput(k));
      }
    });
    let rootBools = [];
    opts.meta.rootKeys.map((k, i) => {
      if (k.type === "boolean") {
        rootBools.push(makeInput(k));
      }
    });

    let rootArrays = [];
    opts.meta.rootArrays.map((k, i) => {
      rootArrays.push(walkNested(k));
    });

    let rootObjects = [];
    opts.meta.rootObjects.map((k, i) => {
      rootObjects.push(walkNested(k));
    });

    let nested = [];
    opts.meta.nested.map((n) => {
      nested.push(walkNested(n));
    });

    let rootKeyDomBool = [];
    let rootKeyDom = [];

    if (rootBools.length > 0) {
      rootKeyDomBool = React.createElement(
        "div",
        {
          key: "root_keys_bools",
          id: "bools",
          className: "root_keys bools obj_grp",
        },
        ...rootBools,
      );
    }

    if (rootKeys.length > 0) {
      rootKeyDom = React.createElement(
        "div",
        {
          key: "root_keys_obj_grp",
          id: "root_keys",
          className: "root_keys keys obj_grp grid sm:grid-cols-2 gap-3",
        },
        ...rootKeys,
      );
    }

    let rootKeyArray = React.createElement(
      "div",
      {
        key: "root_keys_arrays",
        id: "root_keys",
        className: "root_keys arrays grid sm:grid-cols-2 gap-3",
      },
      ...nested,
      ...rootObjects,
      ...rootArrays,
    );

    let editor1 = React.createElement(
      "div",
      {
        key: "root" + opts.baseClass + "1",
        className: "object-wrapper " + opts.baseClass,
      },
      rootKeyDom,
      rootKeyDomBool,
    );

    let editor2 = React.createElement(
      "div",
      {
        key: "root" + opts.baseClass + "2",
        className: "object-wrapper big-gap " + opts.baseClass,
      },
      rootKeyArray,
    );

    return React.createElement(
      "div",
      {
        key: "root" + opts.baseClass + "3",
        className: opts.baseClass,
      },
      editor1,
      editor2,
    );
  };

  makeMeta(props.object, props.opts);

  return (
    <>
      <div className="button-wrap flex gap-3 mb-3">
        {props.opts.backButton && (
          <Button 
            variant="outline" 
            onClick={() => props.opts.backButton.func()}
            className="h-9 border-[#333] bg-[#111] hover:bg-[#222] hover:text-white/90 text-white/80 shadow-sm"
          >
            <ArrowLeft className="h-4 w-4 mr-1" />
            {props.opts.backButton.title}
          </Button>
        )}
        {props.opts.saveButton && !props.hideSaveButton && (
          <Button
            variant="outline"
            onClick={() => props.opts.saveButton(props.object)}
            className="h-9 border-emerald-800/40 bg-[#0c1e0c] text-emerald-400 hover:bg-emerald-900/30 hover:text-emerald-300 shadow-sm font-medium"
          >
            <Save className="h-4 w-4 mr-1" />
            Save
          </Button>
        )}
        {props.opts.deleteButton && (
          <Button
            variant="outline"
            onClick={() => props.opts.deleteButton.func(props.object)}
            className="h-9 border-red-800/40 bg-[#1e0c0c] text-red-400 hover:bg-red-900/30 hover:text-red-300 shadow-sm font-medium ml-auto"
          >
            <Trash2 className="h-4 w-4 mr-1" />
            {props.opts.deleteButton.title}
          </Button>
        )}
      </div>
      {props.opts.title && <div className="editor-title text-xl font-semibold mb-4 text-white/90 border-b border-[#222] pb-2">{props.opts.title}</div>}
      <div className="rounded-lg bg-[#0a0a0a] p-0shadow-md h-fit">
        {makeDom(props.opts)}
      </div>
    </>
  );
};

export default ObjectEditor;
