import React, { useEffect, useState } from "react";
import CustomToggle from "./component/CustomToggle.jsx";
import { v4 as uuidv4 } from "uuid";
import {
  DesktopIcon,
  EnvelopeClosedIcon,
  FrameIcon,
  LockClosedIcon,
} from "@radix-ui/react-icons";
import GLOBAL_STATE from "../state";
import STORE from "../store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";

const useForm = () => {
  const [inputs, setInputs] = useState({});
  const [tokenLogin, setTokenLogin] = useState(false);
  const [errors, setErrors] = useState({});
  const [mode, setMode] = useState(1);
  const [remember, setRememeber] = useState(false);
  const state = GLOBAL_STATE("login");

  const RemoveToken = () => {
    setTokenLogin(false);
    errors["email"] = "";
    setErrors({ ...errors });
    setInputs((inputs) => ({ ...inputs, ["email"]: "" }));
  };

  const GenerateToken = () => {
    let token = uuidv4();
    setTokenLogin(true);

    setErrors({ ...errors });
    setInputs((inputs) => ({ ...inputs, ["email"]: token }));
  };

  const RegisterSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (inputs["email"].length > 320) {
        errors["email"] = "Maximum 320 characters";
        hasErrors = true;
      }

      if (!tokenLogin) {
        if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
          errors["email"] = "Invalid email format";
          hasErrors = true;
        }
      }
    }

    if (!inputs["password"]) {
      errors["password"] = "Password missing";
      hasErrors = true;
    }
    if (!inputs["password2"]) {
      errors["password2"] = "Password confirm missing";
      hasErrors = true;
    }

    if (inputs["password"] !== inputs["password2"]) {
      errors["password2"] = "Passwords do not match";
      hasErrors = true;
    }

    if (inputs["password"]) {
      if (inputs["password"].length < 10) {
        errors["password"] = "Minimum 10 characters";
        hasErrors = true;
      }
      if (inputs["password"].length > 255) {
        errors["password"] = "Maximum 255 characters";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let x = await state.Register(inputs);
    if (x.status === 200) {
      STORE.Cache.Set("default-email", inputs["email"]);
      inputs["password"] = "";
      inputs["password2"] = "";
      setInputs({ ...inputs });
      setMode(1);
    }
    setErrors({});
  };

  const HandleSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    console.dir(inputs);
    if (!inputs["email"] || inputs["email"] === "") {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (!inputs["password"] || inputs["password"] === "") {
      errors["password"] = "Password missing";
      hasErrors = true;
    }

    if (mode === 1) {
      if (!inputs["devicename"] || inputs["devicename"] === "") {
        errors["devicename"] = "Device login name missing";
        hasErrors = true;
      }
    }

    if (mode === 2) {
      if (!inputs["digits"] || inputs["digits"] === "") {
        errors["digits"] = "Authenticator code missing";
        hasErrors = true;
      }

      if (inputs["digits"] && inputs["digits"].length < 6) {
        errors["digits"] = "Code needs to be at least 6 digits";
        hasErrors = true;
      }
    }

    if (mode === 3) {
      if (!inputs["recovery"] || inputs["recovery"] === "") {
        errors["recovery"] = "Recovery code missing";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    await state.Login(inputs, remember);
    setErrors({});
  };
  const EnableSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
        errors["email"] = "Email address format is incorrect";
        hasErrors = true;
      }
    }

    if (!inputs["code"]) {
      errors["code"] = "code missing";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
      ConfirmCode: inputs["code"],
    };

    let x = await state.API_EnableAccount(request);
    if (x.status === 200) {
      inputs["code"] = "";
      setInputs({ ...inputs });
      setMode(6);
    }
    setErrors({});
  };

  const ResetSubmit = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email / Token missing";
      hasErrors = true;
    }

    // if (inputs["email"]) {
    // 	if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
    // 		errors["email"] = "Email address format is incorrect"
    // 		hasErrors = true
    // 	}
    // }

    if (!inputs["password"]) {
      errors["password"] = "Password missing";
      hasErrors = true;
    }

    if (inputs["password"] && inputs["password"].length < 9) {
      errors["password"] =
        "Password needs to be at least 9 characters in length";
      hasErrors = true;
    }

    if (inputs["password"] && inputs["password"].length > 255) {
      errors["password"] = "Password can not be longer then 255 characters";
      hasErrors = true;
    }

    if (!inputs["password2"]) {
      errors["password2"] = "Password confirmation missing";
      hasErrors = true;
    }

    if (inputs["password"] !== inputs["password2"]) {
      errors["password"] = "Passwords do not match";
      hasErrors = true;
    }

    if (!inputs["code"]) {
      errors["code"] = "code missing";
      hasErrors = true;
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
      NewPassword: inputs["password"],
      ResetCode: inputs["code"],
    };

    let x = await state.ResetPassword(request);
    if (x.status === 200) {
      inputs["password"] = "";
      inputs["password2"] = "";
      inputs["code"] = "";
      setInputs({ ...inputs });
      setMode(1);
    }
    setErrors({});
  };

  const GetCode = async () => {
    let errors = {};
    let hasErrors = false;

    if (!inputs["email"]) {
      errors["email"] = "Email missing";
      hasErrors = true;
    }

    if (inputs["email"]) {
      if (!inputs["email"].includes(".") || !inputs["email"].includes("@")) {
        errors["email"] = "Email address format is incorrect";
        hasErrors = true;
      }
    }

    if (hasErrors) {
      setErrors({ ...errors });
      return;
    }

    let request = {
      Email: inputs["email"],
    };
    let status = await state.GetResetCode(request);
    if (status === true) {
      // do we want to do anything more on success ??
    }
    setErrors({});
  };

  const HandleInputChange = (event) => {
    setInputs((inputs) => ({
      ...inputs,
      [event.target.name]: event.target.value,
    }));
  };

  return {
    remember,
    setRememeber,
    inputs,
    setInputs,
    HandleInputChange,
    HandleSubmit,
    errors,
    setMode,
    mode,
    RegisterSubmit,
    GenerateToken,
    tokenLogin,
    ResetSubmit,
    GetCode,
    RemoveToken,
    EnableSubmit,
  };
};

const Login = (props) => {
  const {
    //
    remember,
    setRememeber,
    inputs,
    setInputs,
    HandleInputChange,
    HandleSubmit,
    errors,
    setMode,
    mode,
    RegisterSubmit,
    GenerateToken,
    tokenLogin,
    ResetSubmit,
    GetCode,
    RemoveToken,
    EnableSubmit,
  } = useForm(props);

  const GetDefaults = () => {
    let i = { ...inputs };

    let defaultDeviceName = STORE.Local.getItem("default-device-name");
    if (defaultDeviceName) {
      i["devicename"] = defaultDeviceName;
    }

    let defaultEmail = STORE.Cache.Get("default-email");
    if (defaultEmail) {
      i["email"] = defaultEmail;
    }

    setInputs(i);
  };

  useEffect(() => {
    GetDefaults();
  }, []);

  const EmailOnlyInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <EnvelopeClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-muted-foreground" />
          <Input
            id="email"
            className="pl-10"
            type="email"
            placeholder="Email"
            value={inputs["email"]}
            name="email"
            onChange={HandleInputChange}
          />
        </div>
        {errors["email"] !== "" && (
          <p className="text-sm text-destructive">{errors["email"]}</p>
        )}
      </div>
    );
  };

  const EmailInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <EnvelopeClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="email"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="email"
            placeholder="Email / Token"
            value={inputs["email"]}
            name="email"
            onChange={HandleInputChange}
          />
        </div>
        {errors["email"] !== "" && (
          <p className="text-sm text-red-500">{errors["email"]}</p>
        )}
      </div>
    );
  };

  const DeviceInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <DesktopIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="devicename"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="text"
            placeholder="Device Name"
            value={inputs["devicename"]}
            name="devicename"
            onChange={HandleInputChange}
          />
        </div>
        {errors["devicename"] && (
          <p className="text-sm text-red-500">{errors["devicename"]}</p>
        )}
      </div>
    );
  };
  const NewPasswordInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-muted-foreground" />
          <Input
            id="password"
            className="pl-10"
            type="password"
            placeholder="New Password"
            value={inputs["password"]}
            name="password"
            onChange={HandleInputChange}
          />
        </div>
        {errors["password"] && (
          <p className="text-sm text-destructive">{errors["password"]}</p>
        )}
      </div>
    );
  };

  const PasswordInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="password"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="password"
            placeholder="Password"
            value={inputs["password"]}
            name="password"
            onChange={HandleInputChange}
          />
        </div>
        {errors["password"] && (
          <p className="text-sm text-red-500">{errors["password"]}</p>
        )}
      </div>
    );
  };

  const TwoFactorInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="digits"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="text"
            placeholder="Authenticator Code (optional)"
            value={inputs["digits"]}
            name="digits"
            onChange={HandleInputChange}
          />
        </div>
        {errors["digits"] && (
          <p className="text-sm text-red-500">{errors["digits"]}</p>
        )}
      </div>
    );
  };

  const ConfirmPasswordInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <LockClosedIcon className="absolute left-3 top-2.5 h-5 w-5 text-muted-foreground" />
          <Input
            id="password2"
            className="pl-10"
            type="password"
            placeholder="Confirm Password"
            value={inputs["password2"]}
            name="password2"
            onChange={HandleInputChange}
          />
        </div>
        {errors["password2"] && (
          <p className="text-sm text-destructive">{errors["password2"]}</p>
        )}
      </div>
    );
  };

  const TokenInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <FrameIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="token"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="text"
            placeholder="Token"
            value={inputs["email"]}
            name="email"
            onChange={HandleInputChange}
          />
        </div>
        {inputs["email"] && (
          <Alert variant="destructive" className="mt-2">
            <AlertDescription className="font-semibold">
              SAVE THIS TOKEN!
            </AlertDescription>
          </Alert>
        )}
        {errors["email"] && (
          <p className="text-sm text-red-500">{errors["email"]}</p>
        )}
      </div>
    );
  };

  const CodeInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <FrameIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="code"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="text"
            placeholder="Code"
            name="code"
            onChange={HandleInputChange}
          />
        </div>
        {errors["code"] && (
          <p className="text-sm text-red-500">{errors["code"]}</p>
        )}
      </div>
    );
  };

  const RecoveryInput = () => {
    return (
      <div className="space-y-2">
        <div className="relative">
          <FrameIcon className="absolute left-3 top-2.5 h-5 w-5 text-[#4B7BF5]" />
          <Input
            id="recovery"
            className="pl-10 bg-[#0B0E14] border-[#1a1f2d] text-white focus:ring-[#4B7BF5] focus:border-[#4B7BF5] h-11"
            type="text"
            placeholder="Two Factor Recovery Code"
            value={inputs["recovery"]}
            name="recovery"
            onChange={HandleInputChange}
          />
        </div>
        {errors["recovery"] && (
          <p className="text-sm text-red-500">{errors["recovery"]}</p>
        )}
      </div>
    );
  };

  const LoginForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Welcome back</h1>
          </div>
          {EmailInput()}
          {DeviceInput()}
          {PasswordInput()}
          {TwoFactorInput()}
          <div className="flex items-center space-x-2">
            <CustomToggle
              value={remember}
              label={<span className="text-[#4B7BF5]">Remember Login</span>}
              toggle={() => {
                setRememeber(!remember);
              }}
            />
          </div>
          <Button className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={HandleSubmit}>
            Login
          </Button>
        </CardContent>
      </Card>
    );
  };
  const RegisterAnonForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Anonymous Registration</h1>
          </div>
          <Alert className="border-2 border-red-500 bg-red-500/10">
            <AlertDescription className="font-medium text-red-500">
              Save your login token in a secure place, it is the only form of authentication you have for your account. If you lose the token your account is lost forever.
            </AlertDescription>
          </Alert>
          {TokenInput()}
          {PasswordInput()}
          {ConfirmPasswordInput()}
          <Button className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={RegisterSubmit}>
            Register
          </Button>
        </CardContent>
      </Card>
    );
  };

  const RegisterForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Create your account</h1>
          </div>
          {tokenLogin && TokenInput()}
          {!tokenLogin && EmailInput()}
          {PasswordInput()}
          {ConfirmPasswordInput()}
          <Button className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={RegisterSubmit}>
            Register
          </Button>
        </CardContent>
      </Card>
    );
  };

  const ResetPasswordForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Reset your password</h1>
          </div>
          {EmailInput()}
          {PasswordInput()}
          {ConfirmPasswordInput()}
          {CodeInput()}
          <div className="flex space-x-2">
            <Button variant="outline" className="flex-1 h-11 bg-[#0B0E14] border-[#1a1f2d] text-white hover:bg-[#1a1f2d] hover:text-white" onClick={() => GetCode()}>
              Get Reset Code
            </Button>
            <Button className="flex-1 h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={() => ResetSubmit()}>
              Reset Password
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  };

  const RecoverTwoFactorForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Two-Factor Recovery</h1>
          </div>
          {EmailInput()}
          {PasswordInput()}
          {RecoveryInput()}
          <Button className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={HandleSubmit}>
            Login
          </Button>
        </CardContent>
      </Card>
    );
  };

  const EnableAccountForm = () => {
    return (
      <Card className="w-full max-w-md mx-auto bg-[#0B0E14] border border-[#1a1f2d] shadow-2xl">
        <CardContent className="space-y-6 p-6">
          <div className="text-center mb-2">
            <h1 className="text-lg font-medium text-white/80">Enable your account</h1>
          </div>
          {EmailInput()}
          {CodeInput()}
          <Button className="w-full h-11 bg-[#4B7BF5] hover:bg-[#4B7BF5]/90 text-white" onClick={EnableSubmit}>
            Enable Account
          </Button>
        </CardContent>
      </Card>
    );
  };

  return (
    <div className="w-full flex flex-col items-center justify-center p-4 bg-black">
      <div className="w-full max-w-md space-y-6">
        {mode === 1 && LoginForm()}
        {mode === 2 && RegisterForm()}
        {mode === 4 && ResetPasswordForm()}
        {mode === 3 && RecoverTwoFactorForm()}
        {mode === 5 && RegisterAnonForm()}
        {mode === 6 && EnableAccountForm()}

        <div className="flex flex-wrap items-center justify-center gap-3 mt-4">
          <Button
            variant="ghost"
            onClick={() => setMode(1)}
            className={`h-9 px-4 ${
              mode === 1 
                ? 'text-[#4B7BF5] hover:text-[#4B7BF5] hover:bg-[#4B7BF5]/10' 
                : 'text-white/50 hover:text-white hover:bg-white/5'
            }`}
          >
            Login
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              RemoveToken();
              setMode(2);
            }}
            className={`h-9 px-4 ${
              mode === 2 
                ? 'text-[#4B7BF5] hover:text-[#4B7BF5] hover:bg-[#4B7BF5]/10' 
                : 'text-white/50 hover:text-white hover:bg-white/5'
            }`}
          >
            Register
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              GenerateToken();
              setMode(5);
            }}
            className={`h-9 px-4 ${
              mode === 5 
                ? 'text-[#4B7BF5] hover:text-[#4B7BF5] hover:bg-[#4B7BF5]/10' 
                : 'text-white/50 hover:text-white hover:bg-white/5'
            }`}
          >
            Register Anonymously
          </Button>
          <Button
            variant="ghost"
            onClick={() => setMode(4)}
            className={`h-9 px-4 ${
              mode === 4 
                ? 'text-[#4B7BF5] hover:text-[#4B7BF5] hover:bg-[#4B7BF5]/10' 
                : 'text-white/50 hover:text-white hover:bg-white/5'
            }`}
          >
            Reset Password
          </Button>
          <Button
            variant="ghost"
            onClick={() => setMode(3)}
            className={`h-9 px-4 ${
              mode === 3 
                ? 'text-[#4B7BF5] hover:text-[#4B7BF5] hover:bg-[#4B7BF5]/10' 
                : 'text-white/50 hover:text-white hover:bg-white/5'
            }`}
          >
            2FA Recovery
          </Button>
        </div>
      </div>
    </div>
  );
};

export default Login;
