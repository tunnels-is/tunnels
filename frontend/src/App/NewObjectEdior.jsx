import { Form, FormLabel } from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { useState } from "react"
import GLOBAL_STATE from "../state";
import { Textarea } from "@/components/ui/textarea"

const NewObjectEditor = (props) => {
  const [trigger, setTrigger] = useState({ id: 1 });
  const state = GLOBAL_STATE("object-editor");
  const reload = () => {
    let xx = { ...trigger };
    xx.id += 1;
    setTrigger(xx);
  };

  return (
    <Form>
      {Object.keys(props.obj).map(k => {
        let type = getType(props.obj[k])
        if (type === "array" || type === "object") {
          return
        }

        if (k === "PubKey") {
          return (
            <div key={k} className="mt-4 mt-2">
              <Label className="text-white" type="bool" >{k}</Label>

              <Textarea
                className={"w-full" + state.Theme?.borderColor}
                onChange={(e) => {
                  props.onChange(k, String(e.target.value), type)
                }}
              >
              </Textarea>
            </div>
          )
        }

        if (type === "boolean") {
          return (
            <div key={k} className="mt-4 mt-2">
              <Label className="text-white" type="bool" >{k}</Label>
              <Switch className="ml-2 "
                value={props.obj[k]}
                onCheckedChange={(e) => {
                  props.onChange(k, Boolean(e), type)
                  reload()
                }}
              />
            </div>
          )
        }

        if (type === "string" || type === "number") {
          return (
            <div key={k} className=" mt-2">
              <Label className="text-white" >{k}</Label>
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
    </Form >
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
