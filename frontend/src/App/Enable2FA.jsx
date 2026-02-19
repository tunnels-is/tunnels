import React, { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import QRCode from "react-qr-code";
import { Button } from "@/components/ui/button";
import STORE from "../store";
import GLOBAL_STATE from "../state";
import { Input } from "@/components/ui/input";
import { Lock, KeyRound } from "lucide-react";

const useForm = () => {
  const [inputs, setInputs] = useState({});
  const [errors, setErrors] = useState({});
  const [code, setCode] = useState({});
  const navigate = useNavigate();
  const state = GLOBAL_STATE();

  const HandleSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["digits"]) {
      errors["digits"] = "Authenticator code missing";
      hasErrors = true;
    } else {
      if (inputs["digits"].length < 6) {
        errors["digits"] = "Authenticator code is too short";
        hasErrors = true;
      }
      if (inputs["digits"].length > 6) {
        errors["digits"] = "Authenticator code is too long";
        hasErrors = true;
      }
    }

    if (!inputs["password"]) {
      errors["password"] = "Please enter your password";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let user = STORE.Cache.GetObject("user");
    if (!user) {
      navigate("/login");
    }

    let c = code.Value;
    let firstSplit = c.split("&");
    let secondSplit = firstSplit[1].split("=");
    let secret = secondSplit[1];
    if (secret === "") {
      state?.toggleError("Could not parse authenticator secret");
      setErrors({});
      return;
    }

    inputs.Code = secret;

    let x = await state.callController(null, "POST", "/v3/user/2fa/confirm", inputs, false, false);
    if (x.status === 200) {
      let c = { ...code };
      c.Recovery = x.data.Data;
      setCode(c);
    }

    setErrors({});
  };

  const Get2FACode = async () => {
    let user = STORE.Cache.GetObject("user");
    if (!user) {
      navigate("/login");
    }

    let data = { Email: user.Email };
    let x = await state.API.method("getQRCode", data);
    if (x?.data) {
      setCode(x.data);
    } else {
      state?.toggleError("Unknown error, please try again in a moment");
    }
    setErrors({});
  };

  const HandleInputChange = (event) => {
    setInputs((inputs) => ({ ...inputs, [event.target.name]: event.target.value }));
  };

  return { inputs, HandleInputChange, HandleSubmit, errors, code, Get2FACode };
};

const Enable2FA = () => {
  const { inputs, HandleInputChange, HandleSubmit, errors, code, Get2FACode } = useForm();

  useEffect(() => {
    Get2FACode();
  }, []);

  return (
    <div className="w-full flex flex-col items-center justify-center p-4">
      <div className="w-full max-w-md space-y-6">
        <div className="rounded-lg bg-[#0a0d14]/80 border border-[#1e2433] p-6">
          {code.Value && !code.Recovery && (
            <>
              <div className="qr-code p-4 bg-white w-[260px] m-auto rounded">
                <QRCode
                  className="qr"
                  style={{ height: "auto", maxWidth: "220px", width: "220px" }}
                  value={code.Value}
                  viewBox="0 0 256 256"
                />
              </div>

              <div className="space-y-3 mt-6">
                <div>
                  <label className="text-[10px] text-white/30 uppercase block mb-1">Password</label>
                  <div className="relative">
                    <Lock className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-white/15" />
                    <Input
                      className="h-7 pl-8 text-[12px] border-[#1e2433] bg-transparent"
                      type="password"
                      placeholder="Your account password"
                      value={inputs["password"]}
                      name="password"
                      onChange={HandleInputChange}
                    />
                  </div>
                  {errors["password"] && <p className="text-[11px] text-red-400 mt-1">{errors["password"]}</p>}
                </div>

                <div>
                  <label className="text-[10px] text-white/30 uppercase block mb-1">Authenticator Code</label>
                  <div className="relative">
                    <KeyRound className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-white/15" />
                    <Input
                      className="h-7 pl-8 text-[12px] border-[#1e2433] bg-transparent"
                      type="text"
                      placeholder="6-digit code"
                      value={inputs["digits"]}
                      name="digits"
                      onChange={HandleInputChange}
                    />
                  </div>
                  {errors["digits"] && <p className="text-[11px] text-red-400 mt-1">{errors["digits"]}</p>}
                </div>

                <Button
                  className="w-full text-white bg-emerald-600 hover:bg-emerald-500 h-7 text-[11px]"
                  onClick={HandleSubmit}
                >
                  Confirm
                </Button>

                <div className="pt-3 border-t border-[#1e2433]">
                  <p className="text-[11px] text-white/30 mb-2">
                    Have a recovery code? Enter it below to replace existing 2FA.
                  </p>
                  <div>
                    <label className="text-[10px] text-white/30 uppercase block mb-1">Recovery Code</label>
                    <div className="relative">
                      <KeyRound className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-white/15" />
                      <Input
                        className="h-7 pl-8 text-[12px] border-[#1e2433] bg-transparent"
                        type="text"
                        placeholder="Recovery Code"
                        value={inputs["recovery"]}
                        name="recovery"
                        onChange={HandleInputChange}
                      />
                    </div>
                  </div>
                </div>
              </div>
            </>
          )}

          {code.Recovery && (
            <div className="flex flex-col w-full">
              <span className="text-[11px] text-white/30 font-medium uppercase tracking-wider mb-3">Recovery Codes</span>
              <div className="py-3 px-4 rounded bg-red-500/5 border border-red-500/15 mb-3">
                <p className="text-[11px] text-red-400/80">DO NOT STORE THESE CODES WITH YOUR PASSWORD</p>
              </div>
              <code className="text-[13px] text-white/80 font-mono break-all">{code.Recovery}</code>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Enable2FA;
