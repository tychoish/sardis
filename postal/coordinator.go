package postal

/*
   We need to do something here to read from an iterator, make a job,
   get/maintain a lock, wrap the worker in something that captures the
   output and marks it complete.

   the interface needs to center on the envelapope/record, and include

   - iterator to get docs

   - interface to take the lock (plus update/inc lock)

   - interface to report errors and finish (wrapper function that can
      be canceled, if)

*/
