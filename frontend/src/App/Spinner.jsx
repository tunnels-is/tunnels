import Loader from "react-spinners/ClockLoader";

const Spinner = () => {
    return (
        <Loader
            className="spinner"
            loading={true}
            color={"#FF922D"}
            size={50}
        />
    )
}

export default Spinner;