"use client";

import { type Dispatch, type SetStateAction, type TransitionStartFunction } from "react";

import type { JobItem, ReviewMeta } from "@/lib/api";
import { deleteJob, importReviewJob, rejectReviewJob } from "@/lib/api";

export interface UseReviewBatchActionsDeps {
  selected: JobItem | null;
  meta: ReviewMeta | null;
  moveRunning: boolean;
  selectedJobIds: Set<number>;
  deleteTargetIds: number[] | null;
  setDeleteTargetIds: Dispatch<SetStateAction<number[] | null>>;
  setMessage: Dispatch<SetStateAction<string>>;
  startTransition: TransitionStartFunction;
  persistReview: (options?: { silent?: boolean; successText?: string }) => Promise<boolean>;
  removeJobFromList: (id: number) => void;
  removeJobsFromList: (ids: number[]) => void;
}

export interface ReviewBatchActions {
  handleImport: () => void;
  handleImportSelected: () => void;
  handleDelete: () => void;
  handleDeleteSelected: () => void;
  handleReject: () => void;
  confirmDelete: () => void;
}

export function useReviewBatchActions(deps: UseReviewBatchActionsDeps): ReviewBatchActions {
  const {
    selected,
    meta,
    moveRunning,
    selectedJobIds,
    deleteTargetIds,
    setDeleteTargetIds,
    setMessage,
    startTransition,
    persistReview,
    removeJobFromList,
    removeJobsFromList,
  } = deps;

  const selectedCount = selectedJobIds.size;

  const handleImport = () => {
    if (!selected || !meta) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可审批入库");
      return;
    }
    startTransition(async () => {
      const ok = await persistReview({ silent: true });
      if (!ok) {
        return;
      }
      try {
        setMessage("执行入库...");
        await importReviewJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("入库完成，任务已移出 review 列表");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "入库失败");
      }
    });
  };

  const handleImportSelected = () => {
    if (selectedCount === 0) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可批量审批入库");
      return;
    }
    startTransition(async () => {
      const targetIDs = Array.from(selectedJobIds);
      if (targetIDs.length === 0) {
        return;
      }
      if (selected && meta && selectedJobIds.has(selected.id)) {
        const ok = await persistReview({ silent: true });
        if (!ok) {
          return;
        }
      }
      const successIDs: number[] = [];
      const failures: string[] = [];
      for (let index = 0; index < targetIDs.length; index += 1) {
        const id = targetIDs[index];
        try {
          setMessage(`批量审批中 ${index + 1}/${targetIDs.length}...`);
          await importReviewJob(id);
          successIDs.push(id);
        } catch (error) {
          failures.push(error instanceof Error ? error.message : `任务 #${id} 入库失败`);
        }
      }
      if (successIDs.length > 0) {
        removeJobsFromList(successIDs);
      }
      if (failures.length === 0) {
        setMessage(`批量审批完成，已入库 ${successIDs.length} 项`);
        return;
      }
      if (successIDs.length === 0) {
        setMessage(failures[0] ?? "批量审批失败");
        return;
      }
      setMessage(`已入库 ${successIDs.length} 项，${failures.length} 项失败`);
    });
  };

  const handleDelete = () => {
    if (!selected) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可删除任务");
      return;
    }
    setDeleteTargetIds([selected.id]);
  };

  // handleReject 对应 "打回" 操作: 仅支持单个任务, 把当前 reviewing job 回退
  // 到 failed 状态并清掉 scrape_data, 用户可以回到 /processing 修改影片 ID/variants
  // 后重新 run。批量 reject 暂不支持 (产品决策), 所以这里只针对 selected。
  const handleReject = () => {
    if (!selected) {
      return;
    }
    startTransition(async () => {
      try {
        setMessage("打回任务...");
        await rejectReviewJob(selected.id);
        removeJobFromList(selected.id);
        setMessage("任务已打回，可到文件列表修改影片 ID 后重新 run");
      } catch (error) {
        setMessage(error instanceof Error ? error.message : "打回失败");
      }
    });
  };

  const handleDeleteSelected = () => {
    if (selectedCount === 0) {
      return;
    }
    if (moveRunning) {
      setMessage("媒体库移动进行中，暂不可批量删除");
      return;
    }
    setDeleteTargetIds(Array.from(selectedJobIds));
  };

  const confirmDelete = () => {
    if (!deleteTargetIds || deleteTargetIds.length === 0) {
      return;
    }
    // 防御式守卫: 按钮 disabled 只挡了 "点开对话框" 这一步, 但用户完全可能
    // 在 moveRunning 变 true 之前就已经把对话框打开。此时直接点"删除"会
    // 绕过 UI 锁, 走到 deleteJob → os.Remove, 和迁移中的搬文件撞车。
    // 这里和 handleDelete / handleDeleteSelected 保持同一套消息, 同时把
    // targetIds 清空让对话框关闭, 提示用户当前动作被驳回。
    if (moveRunning) {
      setDeleteTargetIds(null);
      setMessage("媒体库移动进行中，删除已取消");
      return;
    }
    const targetIDs = deleteTargetIds;
    setDeleteTargetIds(null);
    startTransition(async () => {
      const successIDs: number[] = [];
      const failures: string[] = [];
      for (let index = 0; index < targetIDs.length; index += 1) {
        const id = targetIDs[index];
        try {
          setMessage(targetIDs.length > 1 ? `批量删除中 ${index + 1}/${targetIDs.length}...` : "删除任务...");
          await deleteJob(id);
          successIDs.push(id);
        } catch (error) {
          failures.push(error instanceof Error ? error.message : `任务 #${id} 删除失败`);
        }
      }
      if (successIDs.length > 0) {
        removeJobsFromList(successIDs);
      }
      if (failures.length === 0) {
        setMessage(targetIDs.length > 1 ? `已删除 ${successIDs.length} 项` : "任务已删除");
        return;
      }
      if (successIDs.length === 0) {
        setMessage(failures[0] ?? "删除失败");
        return;
      }
      setMessage(`已删除 ${successIDs.length} 项，${failures.length} 项失败`);
    });
  };

  return {
    handleImport,
    handleImportSelected,
    handleDelete,
    handleDeleteSelected,
    handleReject,
    confirmDelete,
  };
}
