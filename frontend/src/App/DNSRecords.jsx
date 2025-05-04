import React, { useEffect } from "react";

import GLOBAL_STATE from "../state";
import ConfigDNSRecordEditor from "./component/ConfigDNSRecordEditor";
import STORE from "../store";

const DNSRecords = () => {
  const state = GLOBAL_STATE("dns");

  let modified = STORE.Cache.GetBool("modified_Config");

  useEffect(() => {
    state.GetBackendState();
  }, []);

  return (
    <div className="dns-page">
      {modified === true && (
        <div className="mb-7 flex gap-[4px] items-center">
          <Button variant="secondary" onClick={() => state.v2_ConfigSave()}>
            Save
          </Button>
          <div className="text-yellow-400 text-xl">
            Your config has un-saved changes
          </div>
        </div>
      )}

      <ConfigDNSRecordEditor />
    </div>
  );
};

export default DNSRecords;
