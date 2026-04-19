"use client";

import { ChevronDown, FileCode2, GripVertical, ScanSearch, Sparkles } from "lucide-react";
import type { RefObject } from "react";

import { Button } from "@/components/ui/button";
import type { PluginEditorCompileResult } from "@/lib/api";

import type { RunAction } from "./plugin-editor-types";

// FloatingMenu: plugin-editor 左上角可拖拽的 "Plugin Builder" 浮动菜单.
// 包括:
//   - 拖拽手柄 (GripVertical + 标题 + Sparkles)
//   - split-action: 主按钮 "编译草稿" + chevron 下拉 (复制 YAML / 导入 YAML
//     / 清空草稿)
//   - 主按钮 "运行调试"
//
// 菜单位置由 floatingMenuPos 控制 (null 时走 default CSS 定位). 拖拽 /
// 位置状态 / handler 全部由 usePluginEditorState hook 负责, 这里只渲染.

export interface FloatingMenuProps {
  compileMenuRef: RefObject<HTMLDivElement | null>;
  floatingMenuPos: { x: number; y: number } | null;
  compileMenuOpen: boolean;
  busyAction: RunAction | "import" | "";
  compileResult: PluginEditorCompileResult | null;
  onPointerDown: (event: React.PointerEvent<HTMLDivElement>) => void;
  onToggleCompileMenu: () => void;
  onRun: (action: RunAction) => void;
  onCopyYAML: () => void;
  onOpenImport: () => void;
  onClearDraft: () => void;
  onCloseCompileMenu: () => void;
}

export function FloatingMenu({
  compileMenuRef,
  floatingMenuPos,
  compileMenuOpen,
  busyAction,
  compileResult,
  onPointerDown,
  onToggleCompileMenu,
  onRun,
  onCopyYAML,
  onOpenImport,
  onClearDraft,
  onCloseCompileMenu,
}: FloatingMenuProps) {
  return (
    <div
      className={`panel plugin-editor-floating-menu ${floatingMenuPos ? "" : "plugin-editor-floating-menu-default"}`}
      style={floatingMenuPos ? { left: `${floatingMenuPos.x}px`, top: `${floatingMenuPos.y}px` } : undefined}
    >
      <div className="plugin-editor-floating-menu-handle" onPointerDown={onPointerDown}>
        <GripVertical size={14} />
        <span>Plugin Builder</span>
        <Sparkles size={14} />
      </div>
      <div className="plugin-editor-floating-menu-actions">
        <div ref={compileMenuRef} className="plugin-editor-split-action">
          <Button
            variant="primary"
            className="plugin-editor-split-action-main"
            onClick={() => onRun("compile")}
            disabled={busyAction !== ""}
            loading={busyAction === "compile"}
            leftIcon={<FileCode2 size={16} />}
          >
            <span>编译草稿</span>
          </Button>
          <Button
            variant="primary"
            className="plugin-editor-split-action-toggle"
            aria-label="展开编译草稿菜单"
            title="展开编译草稿菜单"
            aria-expanded={compileMenuOpen}
            onClick={onToggleCompileMenu}
            disabled={busyAction !== ""}
          >
            <ChevronDown size={14} />
          </Button>
          {compileMenuOpen ? (
            <div className="plugin-editor-split-action-menu">
              <Button
                variant="primary"
                className="plugin-editor-split-action-menu-item"
                onClick={onCopyYAML}
                disabled={!compileResult?.yaml}
              >
                复制 YAML
              </Button>
              <Button
                variant="primary"
                className="plugin-editor-split-action-menu-item"
                onClick={() => {
                  onCloseCompileMenu();
                  onOpenImport();
                }}
                disabled={busyAction !== ""}
              >
                导入 YAML
              </Button>
              <Button
                variant="primary"
                className="plugin-editor-split-action-menu-item"
                onClick={onClearDraft}
              >
                清空草稿
              </Button>
            </div>
          ) : null}
        </div>
        <Button
          variant="primary"
          onClick={() => onRun("scrape")}
          disabled={busyAction !== ""}
          loading={busyAction === "scrape"}
          leftIcon={<ScanSearch size={16} />}
        >
          <span>运行调试</span>
        </Button>
      </div>
    </div>
  );
}
