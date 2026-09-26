import logo from "./assets/img/logo.webp";
import SpotList from "./SpotList";
import useSidebarLayout from "./useSidebarLayout";

function Sidebar() {
  const { sidebarRef, titleRef, listHeight } = useSidebarLayout();

  return (
    <div
      ref={sidebarRef}
      className="md:flex flex-col fixed top-0 left-0 z-10 hidden h-full flex-1 w-100 max-w-[80%] overflow-hidden rounded-r-lg bg-white/70 px-3 shadow-lg backdrop-blur-sm"
      onClick={(event) => event.stopPropagation()}
    >
      <div ref={titleRef} className="flex flex-col items-center mb-4 mt-2 gap-1">
        <img src={logo} alt="Logo" className="w-50" />
        <span className="block text-center text-sm text-gray-700">
          非公式 敷嶋てとら<br />
          聖地巡礼マップ
        </span>
      </div>
      <SpotList
        height={listHeight}
        isCollapsible={false}
        isOpen={true}
        disableInlineVideo={false}
        inlineVideoResetKey={0}
        onRequestOpen={() => undefined}
        onRequestClose={() => undefined}
        focusMapOnSelect={true}
      />
    </div>
  );
}

export default Sidebar;
