on run argv
  set pid to (item 1 of argv) as integer
  set labelText to item 2 of argv
  tell application "System Events" to tell (first application process whose unix id is pid)
    set elements to entire contents of front window
    repeat with e in elements
      try
        if ((name of e is labelText) or (description of e is labelText)) and (exists action "AXPress" of e) then
          perform action "AXPress" of e
          return labelText
        end if
      end try
    end repeat
  end tell
  error "No pressable control: " & labelText
end run
