#define _POSIX_C_SOURCE 200809L // defines the POSIX version, most recent, to implement clock_gettime() function for millis() implementation
#include <stdint.h>
#include <stdio.h>
#include <time.h>
#include <unistd.h>

enum RoverState{
  IDLE,//0
  TELEOP, //1
  AUTO//2
}; // think of enum as describing ints with names EX: RED = 0

enum RoverState currentState = IDLE;// the enum object creation, system will start in IDLE state in the setup() stage
long stateStartTime = 0;
long lastPrintTime = 0;

const long printInterval = 2500; // how often to print the remaining time in milliseconds, in this case every second

// writing functions because the functions must be above the setup and loop

long millis() {
  struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long)ts.tv_sec * 1000LL +
           ts.tv_nsec / 1000000LL;
}

void changeState(enum RoverState newState){ // newState is just another instance for the rover state in function that is used in this function to change the state
  currentState = newState;
  stateStartTime = millis();
  lastPrintTime = millis(); // millis returns the number of milliseconds since the board began running the current program, stores it in stateStartTime so we know when state was entered/started so we can do countdowns based on that time.

  switch (currentState){
    case IDLE:
      printf("IDLE\n");
      fflush(stdout);
      break;
    case TELEOP:

      break;
    case AUTO:

      break;
  }
} 

void setup() {
      changeState(IDLE);
}

void loop() {
  long now = millis(); //how long the program has been running minus the time when the state started which atp is 0 with stateStartTime updated every time the state changes.

  switch(currentState){
    case IDLE:
      if (now - lastPrintTime >= printInterval){
        printf("IDLE tick: %ld ms\n", now);
        fflush(stdout);
        lastPrintTime = now;}
      break;

    case TELEOP:
      break;

    case AUTO:
      break;

  }
}

int main(void){
  setup();
  while(1){loop();}
}