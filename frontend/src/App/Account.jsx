import React, { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import GLOBAL_STATE from "../state";
import dayjs from "dayjs";
import KeyValue from "./component/keyvalue";
import NewTable from "./component/newtable";

const Account = () => {
  const navigate = useNavigate();
  const state = GLOBAL_STATE("account");

  const NavigateTo2fa = () => {
    navigate("/twofactor");
  };

  if (state.User?.Email === "" || !state.User) {
    navigate("/login");
    return;
  }

  useEffect(() => {
    state.GetBackendState();
  }, []);

  state.User?.Tokens?.sort(function (x, y) {
    if (x.Created < y.Created) {
      return 1;
    }
    if (x.Created > y.Created) {
      return -1;
    }
    return 0;
  });

  const generateListTable = (tokens) => {
    let rows = [];
    tokens?.forEach((token) => {
      let current = false;
      if (token.DT === state?.User?.DeviceToken.DT) {
        current = true;
      }

      let row = {};
      row.items = [
        { type: "text", value: current ? token.N + " (this device)" : token.N },
        {
          type: "text",
          value: dayjs(token.Created).format("DD-MM-YYYY HH:mm:ss"),
        },
        {
          type: "text",
          click: () => {
            state.LogoutToken(token);
          },
          value: (
            <div className={`logout clickable`} value={"Logout"}>
              Logout
            </div>
          ),
        },
      ];
      rows.push(row);
    });
    return rows;
  };

  let rows = generateListTable(state.User?.Tokens);
  const headers = [{ value: "Device" }, { value: "Login Date" }, { value: "" }];

  let APIKey = state.getKey("User", "APIKey");

  return (
    <div className="account-page">
      {state?.User && (
        <div className="panel">
          <div className="title">Account</div>

          <KeyValue label={"User"} value={state.User?.Email} />
          <KeyValue
            label={"Last Update"}
            value={dayjs(state.User.Updated).format("DD-MM-YYYY HH:mm:ss")}
          />
          <KeyValue label={"ID"} value={state.User._id} />
          <KeyValue
            label={"API Key"}
            defaultValue={"not set.."}
            value={APIKey}
          />

          {state.User.SubExpiration && (
            <KeyValue
              label={"Subscription Expires"}
              value={dayjs(state.User.SubExpiration).format(
                "DD-MM-YYYY HH:mm:ss",
              )}
            />
          )}

          {state.User.Trial && (
            <KeyValue
              label={"Trial Status"}
              value={state.User.Trial ? "Active" : "Ended"}
            />
          )}

          <KeyValue label={"License"} value={state.User.Key?.Key} />

          <div className="button-and-text-seperator"></div>

          <div className="item full-width-item">
            <div className="button red" onClick={() => state.LogoutAllTokens()}>
              Log Out All Devices
            </div>
          </div>

          <div className="item full-width-item">
            {!state.modifiedUser && (
              <div className="button" onClick={() => state.refreshApiKey()}>
                Re-Generate API Key
              </div>
            )}
            {state.modifiedUser && (
              <div className="button" onClick={() => state.UpdateUser()}>
                Save API Key
              </div>
            )}
          </div>

          <div className="item full-width-item">
            <div className="button" onClick={() => NavigateTo2fa()}>
              Two-Factor Authentication
            </div>
          </div>
          <div className="button-and-text-seperator"></div>
          <div className="item">
            <input
              onChange={(e) => {
                state.UpdateLicenseInput(e.target.value);
              }}
              name="license"
              className="input license"
              placeholder="Insert License Key"
              value={state.LicenseKey}
            />
          </div>

          <div className="item full-width-item" key={state?.LicenseKey}>
            <div className="button" onClick={() => state.ActivateLicense()}>
              Activate Key
            </div>
          </div>
        </div>
      )}

      <NewTable
        tableID={"devices"}
        title={"Logged In Devices"}
        className="logins-list-table"
        background={true}
        header={headers}
        rows={rows}
      />
    </div>
  );
};

export default Account;
