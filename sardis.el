;;; sardis.el --- sardis command runner integration -*- lexical-binding: t -*-

;; Author: sam kleinman
;; Maintainer: tychoish

;;; Commentary:

;; Integration with the sardis command runner/orchestrator.

;;; Code:

(keymap-set hud-core-map "r" #'sardis-run)

;;; Alert style

(with-eval-after-load 'alert
  (alert-define-style
   'sardis
   :title "Notify via sardis"
   :notifier
   (lambda (info)
     (start-process
      "sardis-alert" nil
      "sardis" "notify" "send"
      (or (plist-get info :title) "emacs")
      (or (plist-get info :message) "")))))

;;; Commands
(defun sardis--select-cmd ()
  (annotated-completing-read
   (thread-last (process-lines "sardis" "cmd" "--annotate")
		(mapcar (lambda (it) (split-string it "\t" t "[ \s\t\n]")))
		(mapcar (lambda (cc) (cons (car cc) (cadr cc)))))
   :prompt "sards.cmds =>"
   :require-match nil
   :category 'tychoish/sardis-cmds))

;;;###autoload
(defun sardis-run (&optional sardis-command)
  "Select and run a sardis command in a compile buffer."
  (interactive)
  (let* ((selection (or sardis-command
			(sardis--select-cmd)))
         (task-id (format "sardis-cmd-%s" selection))
         (op-buffer-name (concat "*" task-id "*")))

    (with-current-buffer (get-buffer-create op-buffer-name)
      (add-one-shot-hook :name "task-id"
       :hook 'compilation-finish-functions
       :local t
       :make-unique t
       :args (compilation-buffer message)
       :form (when (< 30 (float-time (time-since (current-idle-time))))
               (alert (string-trim message)
                      :title (format "sardis: %s" selection)
                      :style 'sardis
                      :severity 'moderate
                      :category 'sardis)))

      (save-excursion
        (goto-char (point-min))
	(with-force-write
	    (if (zerop (buffer-size))
		(compilation-insert-annotation (format "# %s\n\n" selection))
	      (compilation-insert-annotation "\n"))
          (compilation-insert-annotation
	   (format "--- [%s] -- %s --\n" selection (format-time-string "%Y-%m-%d %H:%M:%S")))))

      (compilation-start
       (concat "sardis cmd " selection)
       (pa "mode" :is nil)
       (compile-buffer-name op-buffer-name)
       (pa "highlight-regexp" :is nil)
       (pa "continue" :is nil)))))

(provide 'sardis)
;;; sardis.el ends here
