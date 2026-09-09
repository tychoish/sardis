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

;;; Daemons Dashboard Integration

(defcustom sardis-daemons-config-file "~/garen/configs/sardis.system-config.yaml"
  "Path to the Sardis declarative system configuration file."
  :type 'file
  :group 'sardis)

;;;###autoload
(defun sardis-load-daemons-config (&optional file)
  "Load expected system service declarations from Sardis YAML config FILE.
Defaults to `sardis-daemons-config-file'. Registers parsed service
units into `daemons-dash-config-registry' when `daemons-dash-config' is available."
  (interactive)
  (let ((path (expand-file-name (or file sardis-daemons-config-file))))
    (when (and (file-exists-p path) (featurep 'daemons-dash-config))
      (let* ((content (with-temp-buffer
                        (insert-file-contents path)
                        (buffer-string)))
             (parsed (when (fboundp 'yaml-parse-string)
                       (ignore-errors
                         (yaml-parse-string content
                                            :object-type 'plist
                                            :sequence-type 'list
                                            :object-key-type 'keyword))))
             (system (plist-get parsed :system))
             (systemd (plist-get system :systemd))
             (services (plist-get systemd :services)))
        (dolist (s services)
          (let* ((unit (or (plist-get s :unit) (plist-get s :name)))
                 (is-user (plist-get s :user))
                 (disabled (plist-get s :disabled))
                 (provider (if is-user 'systemd-user 'systemd-system))
                 (expected (if disabled 'inactive 'active)))
            (when unit
              (daemons-dash-register-service
               unit
               :name unit
               :provider provider
               :expected-status expected
               :doc (plist-get s :name)))))))))

(with-eval-after-load 'daemons-dash-config
  (sardis-load-daemons-config))

(provide 'sardis)
;;; sardis.el ends here
