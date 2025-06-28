import { Form } from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { useState } from "react"
import GLOBAL_STATE from "../state";
import { Textarea } from "@/components/ui/textarea"
import { SquarePlus } from "lucide-react"
import { SquareMinus } from "lucide-react"

const NewObjectEditor = (props) => {
  const [trigger, setTrigger] = useState({ id: 1 });
  const state = GLOBAL_STATE("object-editor");
  const reload = () => {
    let xx = { ...trigger };
    xx.id += 1;
    setTrigger(xx);
  };

  const walkArray = (key) => {
    return (
      <div key={key} className="mt-4 mt-2">
        <Label className="text-white" type="bool" >{key}</Label>
        {props.obj[key].map((item, i) => {
          return (
            <div className="flex">
              <Input className={"w-[400px]" + state.Theme?.borderColor} onChange={(e) => {
                props.onArrayChange(key, e.target.value, i)
                reload()
              }} type={"array"} value={props.obj[key][i]} />
              <SquareMinus className={"mt-[14px]" + state.Theme?.redIcon} onClick={() => {
                props.obj[key].splice(i, i + 1)
                reload()
              }} />

            </div>
          )
        })}
        <SquarePlus className={"mt-2" + state.Theme?.greenIcon} onClick={() => {
          if (props.obj[key].length > 0) {
            props.obj[key].push(props.obj[key][0])
          } else {
            props.obj[key].push("...")
          }
          reload()
        }} />
      </div>)
  }

  // Helper function to render read-only input fields
  const renderReadOnlyField = (k) => {
    let kk = k
    if (props.opts?.nameMap?.length > 0) {
      kk = props.opts.nameMap[k] ? props.opts.nameMap[k] : k
    }

    if (props.opts?.fields[k] === "hidden") {
      return null
    }

    if (k === "PubKey") {
      return (
        <div key={k} className="mt-4 mt-2">
          <Label className="text-white" type="bool" >{kk}</Label>
          <Textarea
            disabled
            className={"w-full" + state.Theme?.borderColor}
            value={props.obj[k]}
          />
        </div>
      )
    }

    return (
      <div key={k} className=" mt-2">
        <Label className="text-white" >{kk}</Label>
        <Input disabled className={"w-[400px]" + state.Theme?.borderColor}
          value={props.obj[k]} />
      </div>
    )
  }

  // Helper function to render read-only boolean fields
  const renderReadOnlyBoolField = (k) => {
    let kk = k
    if (props.opts?.nameMap?.length > 0) {
      kk = props.opts.nameMap[k] ? props.opts.nameMap[k] : k
    }

    if (props.opts?.fields[k] === "hidden") {
      return null
    }

    return (
      <div key={k} className="mt-4 mt-2">
        <Label className="text-white" type="bool" >{kk}</Label>
        <Switch className="ml-2" disabled checked={props.obj[k]} />
      </div>
    )
  }
  let inputKeys = []
  let boolKeys = []
  let arrayKeys = []
  let readOnlyInputKeys = []
  let readOnlyBoolKeys = []
  let readOnlyArrayKeys = []

  // Helper function to check if a field is read-only
  const isReadOnly = (k) => {
    return props.opts?.fields[k] === "readonly" || 
           k === "_id" || 
           k === "CreatedAt" || 
           k === "Added" || 
           k === "UpdatedAt"
  }

  Object.keys(props.obj).map(k => {
    let type = getType(props.obj[k])
    const readonly = isReadOnly(k)
    
    if (type === "array") {
      if (readonly) {
        readOnlyArrayKeys.push(k)
      } else {
        arrayKeys.push(k)
      }
    } else if (type === "string" || type === "number") {
      if (readonly) {
        readOnlyInputKeys.push(k)
      } else {
        inputKeys.push(k)
      }
    } else if (type === "boolean") {
      if (readonly) {
        readOnlyBoolKeys.push(k)
      } else {
        boolKeys.push(k)
      }
    }
  })

  return (
    <Form>
      {/* Editable input fields */}
      {inputKeys.map(k => {
        let type = getType(props.obj[k])
        let kk = k
        if (props.opts?.nameMap?.length > 0) {
          kk = props.opts.nameMap[k] ? props.opts.nameMap[k] : k
        }

        if (props.opts?.fields[k] === "hidden") {
          return null
        }

        if (k === "PubKey") {
          return (
            <div key={k} className="mt-4 mt-2">
              <Label className="text-white" type="bool" >{kk}</Label>
              <Textarea
                className={"w-full" + state.Theme?.borderColor}
                onChange={(e) => {
                  props.onChange(k, String(e.target.value), type)
                }}
              >
                {props.obj[k]}
              </Textarea>
            </div>
          )
        }
        
        if (type === "string" || type === "number") {
          return (
            <div key={k} className=" mt-2">
              <Label className="text-white" >{kk}</Label>
              <Input className={"w-[400px]" + state.Theme?.borderColor} onChange={(e) => {
                if (type === "number") {
                  props.onChange(k, Number(e.target.value), type)
                } else {
                  props.onChange(k, e.target.value, type)
                }
                reload()
              }} type={type} value={props.obj[k]} />
            </div>
          )
        }
      })}
      
      {/* Editable boolean fields */}
      {boolKeys.map(k => {
        let kk = k
        if (props.opts?.nameMap?.length > 0) {
          kk = props.opts.nameMap[k] ? props.opts.nameMap[k] : k
        }

        if (props.opts?.fields[k] === "hidden") {
          return null
        }
        return (
          <div key={k} className="mt-4 mt-2">
            <Label className="text-white" type="bool" >{kk}</Label>
            <Switch className="ml-2 "
              checked={props.obj[k]}
              onCheckedChange={(e) => {
                props.onChange(k, Boolean(e))
                reload()
              }}
            />
          </div>
        )
      })}
      
      {/* Editable array fields */}
      {arrayKeys.map(k => {
        if (props.opts?.fields[k] === "hidden") {
          return null
        }
        return walkArray(k)
      })}
      
      {/* Read-only input fields */}
      {readOnlyInputKeys.map(k => renderReadOnlyField(k))}
      
      {/* Read-only boolean fields */}
      {readOnlyBoolKeys.map(k => renderReadOnlyBoolField(k))}
      
      {/* Read-only array fields (displayed as disabled) */}
      {readOnlyArrayKeys.map(k => {
        if (props.opts?.fields[k] === "hidden") {
          return null
        }
        let kk = k
        if (props.opts?.nameMap?.length > 0) {
          kk = props.opts.nameMap[k] ? props.opts.nameMap[k] : k
        }
        return (
          <div key={k} className="mt-4 mt-2">
            <Label className="text-white" type="bool" >{kk}</Label>
            {props.obj[k].map((item, i) => {
              return (
                <div key={i} className="flex">
                  <Input disabled className={"w-[400px]" + state.Theme?.borderColor} 
                    value={props.obj[k][i]} />
                </div>
              )
            })}
          </div>
        )
      })}
    </Form>
  )
}

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

export default NewObjectEditor
