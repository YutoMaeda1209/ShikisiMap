import type { RowComponentProps } from "react-window";
import SpotItem, { type ListItemProps } from "../SpotItem";

function MobileSpotItem(props: RowComponentProps<ListItemProps>) {
  return <SpotItem {...props} />;
}

export default MobileSpotItem;
